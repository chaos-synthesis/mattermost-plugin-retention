package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/chaos-synthesis/mattermost-plugin-retention/server/command"
	"github.com/chaos-synthesis/mattermost-plugin-retention/server/store/kvstore"
	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// initRouter initializes the HTTP router for the plugin.
func (p *Plugin) initRouter() *mux.Router {
	router := mux.NewRouter()

	// Middleware to require that the user is logged in
	router.Use(p.MattermostAuthorizationRequired)

	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	apiRouter.HandleFunc("/actions/settings", p.ShowSettings)
	apiRouter.HandleFunc("/settings", p.SaveSettings)

	return router
}

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
// The root URL is currently <siteUrl>/plugins/<plugin id>/api/v1/.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.router.ServeHTTP(w, r)
}

func (p *Plugin) MattermostAuthorizationRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("Mattermost-User-ID")
		if userID == "" {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (p *Plugin) ShowSettings(w http.ResponseWriter, r *http.Request) {
	var payload model.PostActionIntegrationRequest
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, "Failed to decode PostActionIntegrationRequest", http.StatusBadRequest)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			p.API.LogError("Failed to close request body", "err", err)
		}
	}(r.Body)

	userID := r.Header.Get("Mattermost-User-ID")
	userPrefs, err := p.kvStore.GetUserSettings(userID)
	if err != nil {
		p.API.LogError("Failed to decode interaction payload")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ageInDays := 365.
	if userPrefs.PostAgeInDays > 0. {
		ageInDays = userPrefs.PostAgeInDays
	}

	dialog := model.OpenDialogRequest{
		TriggerId: payload.TriggerId,
		URL:       p.GetBundleURL() + "/api/v1/settings", // Endpoint for handling submission
		Dialog: model.Dialog{
			CallbackId:  "basiccallbackid",
			Title:       "Post Retention Settings",
			IconURL:     "http://www.mattermost.org/wp-content/uploads/2016/04/icon.png",
			SubmitLabel: "Save",
			State:       payload.PostId,
			Elements: []model.DialogElement{{
				DisplayName: "Enabled",
				Name:        "enabled",
				Type:        "bool",
				Optional:    true,
				HelpText:    "Enable or disable the post retention policy.",
				Default:     interfaceToString(userPrefs.Enabled),
			}, {
				DisplayName: "Age in days",
				Name:        "age_in_days",
				Type:        "text",
				HelpText:    "Age in days for a post to be considered stale and deleted.",
				MinLength:   1,
				MaxLength:   10,
				Default:     interfaceToString(ageInDays),
			}},
		},
	}

	if err := p.API.OpenInteractiveDialog(dialog); err != nil {
		p.API.LogError("Failed to open interactive dialog", "err", err.Error())
		http.Error(w, "Failed to open dialog", http.StatusInternalServerError)
		return
	}

	response := &model.CommandResponse{}

	p.writeJSON(w, response)
}

func (p *Plugin) SaveSettings(w http.ResponseWriter, r *http.Request) {
	var request model.SubmitDialogRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		p.API.LogError("Failed to decode SubmitDialogRequest", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			p.API.LogError("Failed to close request body", "err", err)
		}
	}(r.Body)

	enabledValue := false
	ageInDaysValue := 0.
	if !request.Cancelled {
		if enabled, ok := request.Submission["enabled"].(bool); ok {
			enabledValue = enabled
		}

		if numberStr, ok := request.Submission["age_in_days"].(string); ok {
			number, parseErr := strconv.ParseFloat(numberStr, 64)
			if number <= 0 || parseErr != nil {
				response := &model.SubmitDialogResponse{
					Errors: map[string]string{"age_in_days": "This must be integer greater than 0"},
				}
				p.writeJSON(w, response)
				return
			}
			ageInDaysValue = number
		}
	}

	userSettings := kvstore.UserSettings{
		UserID:        request.UserId,
		Enabled:       enabledValue,
		PostAgeInDays: ageInDaysValue,
	}

	toastMessage := "Your settings have been saved successfully!"
	err = p.kvStore.SetUserSettings(request.UserId, &userSettings)
	if err != nil {
		p.API.LogError("Failed to set user settings", "err", err.Error())
		toastMessage = "Failed to save your settings. Please contact administrator."
	}

	post := command.CreateStateMessagePost(userSettings, p.GetBundleURL(), toastMessage)
	post.Id = request.State
	post.ChannelId = request.ChannelId

	p.API.UpdateEphemeralPost(request.UserId, post)

	resp := &model.PostActionIntegrationResponse{}
	p.writeJSON(w, resp)
}

// Utility functions

// writeJSON is a helper function to write a JSON response with the appropriate headers and status code.
func (p *Plugin) writeJSON(w http.ResponseWriter, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		p.API.LogError("Failed to write JSON response", "err", err.Error())
	}
}

// GetSiteURL retrieves the SiteURL from the plugin API configuration. It returns an empty string if the SiteURL is not set.
func (p *Plugin) GetSiteURL() string {
	siteURL := ""
	ptr := p.API.GetConfig().ServiceSettings.SiteURL
	if ptr != nil {
		siteURL = *ptr
	}
	return siteURL
}

// GetBundleURL constructs the URL to the plugin's bundle based on the SiteURL and plugin ID.
// It returns an empty string if the SiteURL is not set.
func (p *Plugin) GetBundleURL() string {
	return fmt.Sprintf("%s/plugins/%s", p.GetSiteURL(), manifest.Id)
}

// interfaceToString converts an interface{} value to its string representation.
// It handles common types such as string, float64, and bool, and falls back to using fmt.Sprintf for other types.
func interfaceToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
