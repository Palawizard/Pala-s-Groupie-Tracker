package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Sign-in is delegated to Authentik (https://auth.palawi.fr) through OpenID Connect.
// Required env: OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET, OIDC_REDIRECT_URL.

const oidcStateCookie = "gt_oidc"
const oidcStateTTL = 10 * time.Minute

type oidcClient struct {
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

var (
	oidcOnce   sync.Once
	oidcShared *oidcClient
	oidcErr    error
)

func getOIDC(ctx context.Context) (*oidcClient, error) {
	oidcOnce.Do(func() {
		issuer := strings.TrimSpace(os.Getenv("OIDC_ISSUER"))
		clientID := strings.TrimSpace(os.Getenv("OIDC_CLIENT_ID"))
		secret := strings.TrimSpace(os.Getenv("OIDC_CLIENT_SECRET"))
		redirect := strings.TrimSpace(os.Getenv("OIDC_REDIRECT_URL"))
		if issuer == "" || clientID == "" || secret == "" || redirect == "" {
			oidcErr = errors.New("oidc is not configured (OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET, OIDC_REDIRECT_URL)")
			return
		}
		provider, err := oidc.NewProvider(ctx, issuer)
		if err != nil {
			oidcErr = err
			return
		}
		oidcShared = &oidcClient{
			oauth: &oauth2.Config{
				ClientID:     clientID,
				ClientSecret: secret,
				RedirectURL:  redirect,
				Endpoint:     provider.Endpoint(),
				Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
			},
			verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		}
	})
	if oidcErr != nil {
		// Allow a retry on the next request if discovery failed (e.g. Authentik restarting).
		err := oidcErr
		oidcOnce = sync.Once{}
		oidcErr = nil
		return nil, err
	}
	return oidcShared, nil
}

type oidcState struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Next     string `json:"x"`
}

func randomToken() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// startOIDCLogin redirects the browser to Authentik, remembering where to come back.
func startOIDCLogin(w http.ResponseWriter, r *http.Request, next string) {
	client, err := getOIDC(r.Context())
	if err != nil {
		log.Println("oidc:", err)
		http.Error(w, "Sign-in is temporarily unavailable.", http.StatusServiceUnavailable)
		return
	}
	st := oidcState{State: randomToken(), Nonce: randomToken(), Verifier: oauth2.GenerateVerifier(), Next: next}
	raw, _ := json.Marshal(st)
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    base64.RawURLEncoding.EncodeToString(raw),
		Path:     sessionCookiePath(r),
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(oidcStateTTL.Seconds()),
	})
	url := client.oauth.AuthCodeURL(st.State, oidc.Nonce(st.Nonce), oauth2.S256ChallengeOption(st.Verifier))
	http.Redirect(w, r, url, http.StatusFound)
}

func readOIDCState(w http.ResponseWriter, r *http.Request) (*oidcState, bool) {
	cookie, err := r.Cookie(oidcStateCookie)
	http.SetCookie(w, &http.Cookie{Name: oidcStateCookie, Value: "", Path: sessionCookiePath(r), MaxAge: -1,
		HttpOnly: true, Secure: isSecureRequest(r), SameSite: http.SameSiteLaxMode})
	if err != nil {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil, false
	}
	var st oidcState
	if json.Unmarshal(raw, &st) != nil || st.State == "" {
		return nil, false
	}
	return &st, true
}

// OIDCCallbackHandler finishes the Authentik sign-in and opens a local session.
func OIDCCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		http.Error(w, "Database is not configured.", http.StatusServiceUnavailable)
		return
	}
	st, ok := readOIDCState(w, r)
	if !ok || r.URL.Query().Get("state") != st.State {
		// Expired or replayed attempt: start over cleanly.
		http.Redirect(w, r, withBasePath(r, "/login"), http.StatusSeeOther)
		return
	}
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Redirect(w, r, withBasePath(r, "/"), http.StatusSeeOther)
		return
	}
	client, err := getOIDC(r.Context())
	if err != nil {
		log.Println("oidc:", err)
		http.Error(w, "Sign-in is temporarily unavailable.", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	token, err := client.oauth.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(st.Verifier))
	if err != nil {
		log.Println("oidc exchange:", err)
		http.Error(w, "Sign-in failed.", http.StatusBadGateway)
		return
	}
	rawID, _ := token.Extra("id_token").(string)
	idToken, err := client.verifier.Verify(ctx, rawID)
	if err != nil || idToken.Nonce != st.Nonce {
		log.Println("oidc id_token:", err)
		http.Error(w, "Sign-in failed.", http.StatusBadGateway)
		return
	}
	var claims struct {
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	_ = idToken.Claims(&claims)
	name := claims.PreferredUsername
	if name == "" {
		name = claims.Name
	}
	user, err := appStore.UpsertOIDCUser(ctx, idToken.Subject, claims.Email, name)
	if err != nil {
		log.Println("oidc user:", err)
		http.Error(w, "Sign-in failed.", http.StatusInternalServerError)
		return
	}
	if err := createSession(w, r, user.ID); err != nil {
		http.Error(w, "Sign-in failed.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, resolveNextURL(st.Next, r), http.StatusSeeOther)
}
