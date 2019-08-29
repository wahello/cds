package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/ovh/cds/sdk"

	"github.com/ovh/cds/engine/api/authentication"
	"github.com/ovh/cds/engine/api/authentication/builtin"
	"github.com/ovh/cds/engine/api/user"
	"github.com/ovh/cds/engine/service"
)

func (api *API) getConsumersByUserHandler() service.Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		vars := mux.Vars(r)

		username := vars["permUsername"]

		var u *sdk.AuthentifiedUser
		var err error
		if username == "me" {
			u, err = user.LoadByID(ctx, api.mustDB(), getAPIConsumer(ctx).AuthentifiedUserID)
		} else {
			u, err = user.LoadByUsername(ctx, api.mustDB(), username)
		}
		if err != nil {
			return err
		}

		cs, err := authentication.LoadConsumersByUserID(ctx, api.mustDB(), u.ID,
			authentication.LoadConsumerOptions.Default)
		if err != nil {
			return err
		}

		return service.WriteJSON(w, cs, http.StatusOK)
	}
}

func (api *API) postConsumerByUserHandler() service.Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		var reqData sdk.AuthConsumer
		if err := service.UnmarshalBody(r, &reqData); err != nil {
			return err
		}
		if err := reqData.IsValid(); err != nil {
			return err
		}

		consumer := getAPIConsumer(ctx)

		// Create the new built in consumer from request data
		newConsumer, token, err := builtin.NewConsumer(api.mustDB(), reqData.Name, reqData.Description,
			consumer, reqData.GroupIDs, reqData.Scopes)
		if err != nil {
			return err
		}
		if err := authentication.LoadConsumerOptions.Default(ctx, api.mustDB(), newConsumer); err != nil {
			return err
		}

		return service.WriteJSON(w, sdk.AuthConsumerCreateResponse{
			Token:    token,
			Consumer: newConsumer,
		}, http.StatusCreated)
	}
}

func (api *API) deleteConsumerByUserHandler() service.Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		vars := mux.Vars(r)

		username := vars["permUsername"]
		consumerID := vars["permConsumerID"]

		tx, err := api.mustDB().Begin()
		if err != nil {
			return sdk.WithStack(err)
		}
		defer tx.Rollback() // nolint

		var u *sdk.AuthentifiedUser
		if username == "me" {
			u, err = user.LoadByID(ctx, tx, getAPIConsumer(ctx).AuthentifiedUserID)
		} else {
			u, err = user.LoadByUsername(ctx, tx, username)
		}
		if err != nil {
			return err
		}

		consumer, err := authentication.LoadConsumerByID(ctx, tx, consumerID)
		if err != nil {
			return err
		}
		if consumer.Type != sdk.ConsumerBuiltin || consumer.AuthentifiedUserID != u.ID {
			return sdk.WithStack(sdk.ErrForbidden)
		}

		if err := authentication.DeleteConsumerByID(tx, consumer.ID); err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return sdk.WithStack(err)
		}

		return service.WriteJSON(w, nil, http.StatusOK)
	}
}

func (api *API) postConsumerRegenByUserHandler() service.Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		vars := mux.Vars(r)

		consumerID := vars["permConsumerID"]

		var req sdk.AuthConsumerRegenRequest
		if err := service.UnmarshalBody(r, &req); err != nil {
			return err
		}

		tx, err := api.mustDB().Begin()
		if err != nil {
			return sdk.WithStack(err)
		}
		defer tx.Rollback()

		consumer, err := authentication.LoadConsumerByID(ctx, tx, consumerID) // Load the consumer from the input
		if err != nil {
			return err
		}
		consumer.IssuedAt = time.Now()                                      // Update the IAT attribute
		if err := authentication.UpdateConsumer(tx, consumer); err != nil { // Update the updated value in database
			return err
		}
		jws, err := builtin.NewSigninConsumerToken(consumer) // Regen a new jws (signin token)
		if err != nil {
			return err
		}

		if req.RevokeSessions {
			sessions, err := authentication.LoadSessionsByConsumerIDs(ctx, tx, []string{consumer.ID}) // Find all the sessions
			if err != nil {
				return err
			}
			for _, s := range sessions { // Now remove all current sessions for the consumer
				if err := authentication.DeleteSessionByID(tx, s.ID); err != nil {
					return err
				}
			}
		}

		if err := tx.Commit(); err != nil {
			return sdk.WithStack(err)
		}

		return service.WriteJSON(w, sdk.AuthConsumerCreateResponse{
			Token:    jws,
			Consumer: consumer,
		}, http.StatusOK)
	}
}

func (api *API) getSessionsByUserHandler() service.Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		session := getAuthSession(ctx)

		vars := mux.Vars(r)

		username := vars["permUsername"]
		u, err := user.LoadByUsername(ctx, api.mustDB(), username)
		if err != nil {
			return err
		}

		cs, err := authentication.LoadConsumersByUserID(ctx, api.mustDB(), u.ID)
		if err != nil {
			return err
		}

		ss, err := authentication.LoadSessionsByConsumerIDs(ctx, api.mustDB(), sdk.AuthConsumersToIDs(cs))
		if err != nil {
			return err
		}

		// Set extra data on sessions
		for i := range ss {
			if ss[i].ID == session.ID {
				ss[i].Current = true
			}
		}

		return service.WriteJSON(w, ss, http.StatusOK)
	}
}

func (api *API) deleteSessionByUserHandler() service.Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		vars := mux.Vars(r)

		username := vars["permUsername"]
		sessionID := vars["permSessionID"]

		tx, err := api.mustDB().Begin()
		if err != nil {
			return sdk.WithStack(err)
		}
		defer tx.Rollback() // nolint

		u, err := user.LoadByUsername(ctx, api.mustDB(), username)
		if err != nil {
			return err
		}

		cs, err := authentication.LoadConsumersByUserID(ctx, api.mustDB(), u.ID)
		if err != nil {
			return err
		}

		session, err := authentication.LoadSessionByID(ctx, tx, sessionID)
		if err != nil {
			return err
		}
		var found bool
		for i := range cs {
			if cs[i].ID == session.ConsumerID {
				found = true
				break
			}
		}
		if !found {
			return sdk.WithStack(sdk.ErrForbidden)
		}

		if err := authentication.DeleteSessionByID(tx, session.ID); err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return sdk.WithStack(err)
		}

		return service.WriteJSON(w, nil, http.StatusOK)
	}
}
