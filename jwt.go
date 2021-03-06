// Copyright 2019 Luca Stasio <joshuagame@gmail.com>
// Copyright 2019 IT Resources s.r.l.
//
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// Package sgul defines common structures and functionalities for applications.
// jwt.go defines commons for jwt Authorization.
package sgul

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/middleware"

	jwt "github.com/dgrijalva/jwt-go"
)

// Principal defines the struct registered into the Context
// representing the authenticated user information from the JWT Token.
type Principal struct {
	Username string
	Role     string
}

type ctxKey int

const ctxPrincipalKey ctxKey = iota

// ErrPrincipalNotInContext is returned if there is no Principal in the request context.
var ErrPrincipalNotInContext = errors.New("No Principal in request context")

// jwtAuthorize will authorize the incoming user against input roles.
// if the user is authorized, a Principal struct will be set in request context
// for later use in the request mgmtr chain.
//func jwtAuthorize(roles []string, next http.Handler) http.HandlerFunc {
func jwtAuthorize(enforcer RolesEnforcer, next http.Handler) http.HandlerFunc {
	conf := GetConfiguration().API.Security
	secret := []byte(conf.Jwt.Secret)
	return func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		trimmedAuth := strings.Fields(authorization)

		// Trim out Bearer from Authorization Header
		if authorization == "" || len(trimmedAuth) == 0 {
			RenderError(w,
				NewHTTPError(
					errors.New("Unauthorized"),
					http.StatusUnauthorized, "Unauthorized user",
					middleware.GetReqID(r.Context())))
			return
		}

		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(trimmedAuth[1], claims,
			func(token *jwt.Token) (interface{}, error) {
				return secret, nil
			})
		if err != nil {
			RenderError(w,
				NewHTTPError(
					errors.New("Unauthorized"),
					http.StatusUnauthorized, "Unauthorized user",
					middleware.GetReqID(r.Context())))
			return
		}

		principal := Principal{
			Username: claims["sub"].(string),
			Role:     claims["auth"].(string),
		}

		// check roles authorization: 403 Forbidden iff check fails
		//if !ContainsString(roles, principal.Role) {
		if !enforcer.Enforce(r.Context(), principal.Role, r.URL.Path, r.Method) {
			fmt.Printf("error -> %s", errors.New("Forbidden"))
			RenderError(w,
				NewHTTPError(
					errors.New("Forbidden"),
					http.StatusForbidden, "Forbidden resource for the user",
					middleware.GetReqID(r.Context())))
			return
		}

		ctx := context.WithValue(r.Context(), ctxPrincipalKey, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// JWTAuthorizer is the JWT authentication middleware to use on mux (a. e. Chi router or Groups).
//func JWTAuthorizer(roles []string) func(next http.Handler) http.Handler {
func JWTAuthorizer(enforcer RolesEnforcer) func(next http.Handler) http.Handler {
	jwtAuthorizer := func(next http.Handler) http.Handler {
		// return http.HandlerFunc(jwtAuthorize(roles, next))
		if enforcer == nil {
			enforcer = &MatchAllEnforcer{}
		}
		return http.HandlerFunc(jwtAuthorize(enforcer, next))
	}
	return jwtAuthorizer
}

// JWTRouteAuthorizer is the JWT authentication middleware to use on single route (a.e. Chi router get, post, ...).
//func JWTRouteAuthorizer(roles []string) func(next http.HandlerFunc) http.HandlerFunc {
func JWTRouteAuthorizer(enforcer RolesEnforcer) func(next http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		//return jwtAuthorize(roles, next)
		if enforcer == nil {
			enforcer = &MatchAllEnforcer{}
		}
		return jwtAuthorize(enforcer, next)
	}

}

// GetPrincipal return the user authenticated Princiapl information from the request context.
func GetPrincipal(ctx context.Context) (Principal, error) {
	if principal, ok := ctx.Value(ctxPrincipalKey).(Principal); ok {
		return principal, nil
	}
	return Principal{}, ErrPrincipalNotInContext
}
