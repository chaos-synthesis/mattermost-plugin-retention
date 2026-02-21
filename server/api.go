package main

import (
	"encoding/json"
	"io"
	"net/http"

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

	apiRouter.HandleFunc("/settings", p.Settings)

	return router
}

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
// The root URL is currently <siteUrl>/plugins/com.mattermost.plugin-starter-template/api/v1/. Replace com.mattermost.plugin-starter-template with the plugin ID.
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

func (p *Plugin) Settings(w http.ResponseWriter, r *http.Request) {
	//if _, err := w.Write([]byte("Hello, world!")); err != nil {
	//	p.API.LogError("Failed to write response", "error", err)
	//	http.Error(w, err.Error(), http.StatusInternalServerError)
	//}

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

	if !request.Cancelled {
		number, ok := request.Submission["age_in_days"].(float32)
		if ok {
			if number <= 0 {
				response := &model.SubmitDialogResponse{
					Errors: map[string]string{
						"age_in_days": "This must be greater than 0",
					},
				}
				p.writeJSON(w, response)
				return
			}
		}
	}

	err = p.kvStore.SetUserSettings(request.UserId, &kvstore.UserSettings{
		UserID:        request.UserId,
		Enabled:       request.Submission["enabled"].(bool),
		PostAgeInDays: request.Submission["age_in_days"].(float32),
	})
	if err != nil {
		p.API.LogError("Failed to set user settings", "err", err.Error())
		return
	}

	// Send the toast message using the plugin API
	options := model.SendToastMessageOptions{
		Position: "bottom-right",
	}

	if err := p.client.Frontend.SendToastMessage(args.UserId, connectionID, message, options); err != nil {
		errorMessage := "Failed to send toast notification"
		p.API.LogError(errorMessage, "err", err.Error())
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         errorMessage,
		}
	}

	//user, appErr := p.API.GetUser(request.UserId)
	//if appErr != nil {
	//	p.API.LogError("Failed to get user for dialog", "err", appErr.Error())
	//	w.WriteHeader(http.StatusOK)
	//	return
	//}
	//
	//msg := "@%v submitted an Interative Dialog"
	//if request.Cancelled {
	//	msg = "@%v canceled an Interative Dialog"
	//}
	//
	//rootPost, appErr := p.API.CreatePost(&model.Post{
	//	UserId:    p.botID,
	//	ChannelId: request.ChannelId,
	//	Message:   fmt.Sprintf(msg, user.Username),
	//})
	//if appErr != nil {
	//	p.API.LogError("Failed to post handleDialog1 message", "err", appErr.Error())
	//	return
	//}
	//
	//if !request.Cancelled {
	//	// Don't post the email address publicly
	//	if request.Submission[dialogElementNameEmail] != nil {
	//		request.Submission[dialogElementNameEmail] = "xxxxxxxxxxx"
	//	}
	//	if _, appErr = p.API.CreatePost(&model.Post{
	//		UserId:    p.botID,
	//		ChannelId: request.ChannelId,
	//		RootId:    rootPost.Id,
	//		Message:   "Data:",
	//		Type:      "custom_demo_plugin",
	//		Props:     request.Submission,
	//	}); appErr != nil {
	//		p.API.LogError("Failed to post handleDialog1 message", "err", appErr.Error())
	//		return
	//	}
	//}

	w.WriteHeader(http.StatusOK)
}

func (p *Plugin) writeJSON(w http.ResponseWriter, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		p.API.LogError("Failed to write JSON response", "err", err.Error())
	}
}
