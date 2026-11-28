package router

import (
	"net/http"

	"github.com/gorilla/mux"
)

type DonationHandler interface {
	GetDonationSettings(w http.ResponseWriter, r *http.Request)
	SendDonation(w http.ResponseWriter, r *http.Request)
	GetMySentDonations(w http.ResponseWriter, r *http.Request)
	GetMyReceivedDonations(w http.ResponseWriter, r *http.Request)
}

type Router struct {
	Donation DonationHandler
}

func RegisterPublicRoutes(public *mux.Router, deps Router) {
	public.HandleFunc("/donations/settings", deps.Donation.GetDonationSettings).Methods(http.MethodGet, http.MethodOptions)
}

func RegisterRoutes(private *mux.Router, deps Router) {
	private.HandleFunc("/authors/{author_id}/donations", deps.Donation.SendDonation).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/profile/me/donations/sent", deps.Donation.GetMySentDonations).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/profile/me/donations/received", deps.Donation.GetMyReceivedDonations).Methods(http.MethodGet, http.MethodOptions)
}
