package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/dolmatovDan/cofffe_shop/data"
	protos "github.com/dolmatovDan/gRPC/currency"
	"github.com/gorilla/mux"
)

// http.Handler
type Products struct {
	l  *log.Logger
	v  *data.Validation
	cc protos.CurrencyClient
}

// NewProducts creates a products handler with the given logger
func NewProducts(l *log.Logger, v *data.Validation, cc protos.CurrencyClient) *Products {
	return &Products{l, v, cc}
}

// GenericError is a generic error message returned by a server
type GenericError struct {
	Message string `json:"message"`
}

// ValidationError is a collection of validation error messages
type ValidationError struct {
	Messages []string `json:"messages"`
}

func getProductID(r *http.Request) int {
	// parse the product id from the url
	vars := mux.Vars(r)

	// convert the id into an integer and return
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		// should never happen
		panic(err)
	}

	return id
}

type KeyProduct struct{}
