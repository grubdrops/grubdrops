package api

import (
	"cmp"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aalejandrofer/grubdrops/internal/store"
)

// needsLinkDropJSON is one drop in GET /api/needs-link: watched in full, not
// claimed because the platform wants a linked game account. Expired is set
// once the campaign ended and the platform's inventory stopped listing it.
type needsLinkDropJSON struct {
	AccountID    string  `json:"account_id"`
	AccountName  string  `json:"account_name"`
	Platform     string  `json:"platform"`
	BenefitID    string  `json:"benefit_id"`
	BenefitName  string  `json:"benefit_name"`
	ImageURL     string  `json:"image_url"`
	CampaignID   string  `json:"campaign_id"`
	CampaignName string  `json:"campaign_name"`
	Game         string  `json:"game"`
	LinkURL      string  `json:"link_url"`
	EndsAt       *string `json:"ends_at"`
	Expired      bool    `json:"expired"`
}

func (s *server) routesNeedsLink(r chi.Router) {
	r.Get("/needs-link", s.getNeedsLink)
}

// getNeedsLink lists the completed drops refused for a missing link that
// are not claimed yet, by account, latest campaign end first.
func (s *server) getNeedsLink(w http.ResponseWriter, r *http.Request) {
	if s.d.NeedsLink == nil {
		writeErr(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	ctx := r.Context()
	rows, err := s.d.NeedsLink.Unclaimed(ctx)
	if err != nil {
		slog.Warn("api: list needs-link drops failed", "kind", "error", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal")
		return
	}
	names := map[string]string{}
	if accts, err := s.d.Q.ListAllAccounts(ctx); err == nil {
		for _, a := range accts {
			names[a.ID] = store.AccountName(a)
		}
	}
	now := s.overviewNow()
	out := []needsLinkDropJSON{}
	for _, d := range rows {
		out = append(out, needsLinkDropJSON{
			AccountID: d.AccountID, AccountName: cmp.Or(names[d.AccountID], d.AccountID), Platform: d.Platform,
			BenefitID: d.BenefitID, BenefitName: d.BenefitName, ImageURL: d.ImageURL,
			CampaignID: d.CampaignID, CampaignName: d.CampaignName, Game: d.Game,
			LinkURL: d.LinkURL, EndsAt: rfc3339(d.EndsAt), Expired: d.Expired(now),
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"drops": out})
}
