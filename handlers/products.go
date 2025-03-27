package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"github.com/dolmatovDan/cofffe_shop/data"
	protos "github.com/dolmatovDan/gRPC/currency"
	"github.com/gorilla/mux"
)

// http.Handler
type Products struct {
	l *log.Logger
	cc protos.CurrencyClient
}

// NewProducts creates a products handler with the given logger
func NewProducts(l *log.Logger, cc protos.CurrencyClient) *Products {
	return &Products{l, cc}
}

// return product from data
func (p *Products) GetProducts(rw http.ResponseWriter, r *http.Request) {
	p.l.Println("Handle GET Products")

	lp := data.GetProducts()

	// get exchange from gRPC client
	rr := &protos.RateRequest{
		Base: protos.Currencies(protos.Currencies_value["EUR"]),
		Destination: protos.Currencies(protos.Currencies_value["GBP"]),
	}
	resp, err := p.cc.GetRate(context.Background(), rr)
	if err != nil {
		p.l.Println("[Error] error getting new rate", err)
		http.Error(rw, "Error getting currency rate", http.StatusBadRequest)
		return
	}
	p.l.Printf("Resp %#v", resp)
	
	for _, p := range lp {
		p.Price *= resp.Rate
	}

	err = lp.ToJSON(rw)
	if err != nil {
		http.Error(rw, "Unable to marshal json", http.StatusInternalServerError)
	}
}

func (p *Products) AddProduct(rw http.ResponseWriter, r *http.Request) {
	p.l.Println("Handle POST Product")

	prod := r.Context().Value(KeyProduct{}).(data.Product)
	data.AddProduct(&prod)
}

func (p Products) UpdateProducts(rw http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(rw, "Unable to convert id", http.StatusBadRequest)
		return
	}

	p.l.Println("Handle PUT Product", id)
	prod := r.Context().Value(KeyProduct{}).(data.Product)

	err = data.UpdateProduct(id, &prod)
	if err == data.ErrProductNotFound {
		http.Error(rw, "Product not found", http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(rw, "Product not found", http.StatusInternalServerError)
		return
	}
}

type KeyProduct struct{}

func (p Products) MiddlewareValidateProduct(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		prod := data.Product{}

		err := prod.FromJSON(r.Body)
		if err != nil {
			p.l.Println("[ERROR] deserializing product", err)
			http.Error(rw, "Error reading product", http.StatusBadRequest)
			return
		}

		// add the product to the context
		ctx := context.WithValue(r.Context(), KeyProduct{}, prod)
		r = r.WithContext(ctx)

		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(rw, r)
	})
}
