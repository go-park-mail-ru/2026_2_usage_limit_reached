package router

import (
	"net/http"

	"github.com/gorilla/mux"
)

type PostHandler interface {
	Feed(w http.ResponseWriter, r *http.Request)
	UserPosts(w http.ResponseWriter, r *http.Request)
	MyPosts(w http.ResponseWriter, r *http.Request)
	MyDrafts(w http.ResponseWriter, r *http.Request)
	ViewPost(w http.ResponseWriter, r *http.Request)
	NewPost(w http.ResponseWriter, r *http.Request)
	EditPost(w http.ResponseWriter, r *http.Request)
	DeletePost(w http.ResponseWriter, r *http.Request)
	LikePost(w http.ResponseWriter, r *http.Request)
	UnlikePost(w http.ResponseWriter, r *http.Request)
}

type AttachmentsHandler interface {
	UploadAttachment(w http.ResponseWriter, r *http.Request)
	GetAttachment(w http.ResponseWriter, r *http.Request)
}

type Handlers struct {
	Post        PostHandler
	Attachments AttachmentsHandler
}

func RegisterRoutes(public *mux.Router, private *mux.Router, deps Handlers) {
	public.HandleFunc("/feed", deps.Post.Feed).Methods(http.MethodGet, http.MethodOptions)
	public.HandleFunc("/posts/{id}", deps.Post.ViewPost).Methods(http.MethodGet, http.MethodOptions)
	public.HandleFunc("/users/{username}/posts", deps.Post.UserPosts).Methods(http.MethodGet, http.MethodOptions)
	public.HandleFunc("/attachments/{id}", deps.Attachments.GetAttachment).Methods(http.MethodGet, http.MethodHead, http.MethodOptions)

	private.HandleFunc("/profile/me/posts", deps.Post.MyPosts).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/profile/me/drafts", deps.Post.MyDrafts).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/posts", deps.Post.NewPost).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/posts/{id}", deps.Post.EditPost).Methods(http.MethodPut, http.MethodOptions)
	private.HandleFunc("/posts/{id}", deps.Post.DeletePost).Methods(http.MethodDelete, http.MethodOptions)
	private.HandleFunc("/posts/{id}/like", deps.Post.LikePost).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/posts/{id}/like", deps.Post.UnlikePost).Methods(http.MethodDelete, http.MethodOptions)
	private.HandleFunc("/attachments", deps.Attachments.UploadAttachment).Methods(http.MethodPost, http.MethodOptions)
}
