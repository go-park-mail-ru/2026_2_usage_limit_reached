package router

import (
	"net/http"

	"github.com/gorilla/mux"
)

type AuthHandler interface {
	Register(w http.ResponseWriter, r *http.Request)
	Login(w http.ResponseWriter, r *http.Request)
	Logout(w http.ResponseWriter, r *http.Request)
}

type ProfileHandler interface {
	GetMe(w http.ResponseWriter, r *http.Request)
	GetPublicProfile(w http.ResponseWriter, r *http.Request)
	UpdateMyProfile(w http.ResponseWriter, r *http.Request)
	UploadAvatar(w http.ResponseWriter, r *http.Request)
	DeleteAvatar(w http.ResponseWriter, r *http.Request)
}

type CSRFHandler interface {
	GetToken(w http.ResponseWriter, r *http.Request)
}

type AuthorHandler interface {
	SearchAuthors(w http.ResponseWriter, r *http.Request)
	BecomeAuthor(w http.ResponseWriter, r *http.Request)
	GetMeAuthor(w http.ResponseWriter, r *http.Request)
	EditAuthor(w http.ResponseWriter, r *http.Request)
}

type Handlers struct {
	Auth    AuthHandler
	Profile ProfileHandler
	CSRF    CSRFHandler
	Author  AuthorHandler
}

func RegisterRoutes(public *mux.Router, private *mux.Router, deps Handlers) {
	public.HandleFunc("/signup", deps.Auth.Register).Methods(http.MethodPost, http.MethodOptions)
	public.HandleFunc("/login", deps.Auth.Login).Methods(http.MethodPost, http.MethodOptions)
	public.HandleFunc("/csrf", deps.CSRF.GetToken).Methods(http.MethodGet, http.MethodOptions)
	public.HandleFunc("/users/{username}", deps.Profile.GetPublicProfile).Methods(http.MethodGet, http.MethodOptions)
	public.HandleFunc("/authors", deps.Author.SearchAuthors).Methods(http.MethodGet, http.MethodOptions)

	private.HandleFunc("/logout", deps.Auth.Logout).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/profile/me", deps.Profile.GetMe).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/profile/me", deps.Profile.UpdateMyProfile).Methods(http.MethodPut, http.MethodOptions)
	private.HandleFunc("/profile/me/avatar", deps.Profile.UploadAvatar).Methods(http.MethodPut, http.MethodOptions)
	private.HandleFunc("/profile/me/avatar", deps.Profile.DeleteAvatar).Methods(http.MethodDelete, http.MethodOptions)
	private.HandleFunc("/authors/me", deps.Author.BecomeAuthor).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/authors/me", deps.Author.GetMeAuthor).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/authors/me", deps.Author.EditAuthor).Methods(http.MethodPut, http.MethodOptions)
}
