package router

import (
	"net/http"

	"github.com/gorilla/mux"
)

type DonationHandler interface {
	SendDonation(w http.ResponseWriter, r *http.Request)
	GetMySentDonations(w http.ResponseWriter, r *http.Request)
	GetMyReceivedDonations(w http.ResponseWriter, r *http.Request)
}

type Handlers struct {
	Donation DonationHandler
}

func RegisterRoutes(private *mux.Router, deps Handlers) {
	private.HandleFunc("/authors/{author_id}/donations", deps.Donation.SendDonation).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/profile/me/donations/sent", deps.Donation.GetMySentDonations).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/profile/me/donations/received", deps.Donation.GetMyReceivedDonations).Methods(http.MethodGet, http.MethodOptions)
}
