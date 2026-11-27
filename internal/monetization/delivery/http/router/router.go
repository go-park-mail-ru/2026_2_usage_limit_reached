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

type SubscriptionHandler interface {
	GetMySubscriptionTiers(w http.ResponseWriter, r *http.Request)
	CreateSubscriptionTier(w http.ResponseWriter, r *http.Request)
	UpdateSubscriptionTier(w http.ResponseWriter, r *http.Request)
	GetSubscriptionOptions(w http.ResponseWriter, r *http.Request)
	CreateSubscription(w http.ResponseWriter, r *http.Request)
}

type Router struct {
	Donation     DonationHandler
	Subscription SubscriptionHandler
}

func RegisterRoutes(public *mux.Router, private *mux.Router, deps Router) {
	private.HandleFunc("/authors/{author_id}/donations", deps.Donation.SendDonation).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/profile/me/donations/sent", deps.Donation.GetMySentDonations).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/profile/me/donations/received", deps.Donation.GetMyReceivedDonations).Methods(http.MethodGet, http.MethodOptions)

	if deps.Subscription == nil {
		return
	}

	public.HandleFunc("/authors/{author_id}/subscription-options", deps.Subscription.GetSubscriptionOptions).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/profile/me/tiers", deps.Subscription.GetMySubscriptionTiers).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/profile/me/tiers", deps.Subscription.CreateSubscriptionTier).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/profile/me/tiers/{id}", deps.Subscription.UpdateSubscriptionTier).Methods(http.MethodPatch, http.MethodOptions)
	private.HandleFunc("/authors/{author_id}/subscriptions", deps.Subscription.CreateSubscription).Methods(http.MethodPost, http.MethodOptions)
}
