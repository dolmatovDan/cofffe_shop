package handlers

import (
	"net/http"

	"github.com/dolmatovDan/cofffe_shop/data"
)

func (p *Products) ListAll(rw http.ResponseWriter, r *http.Request) {
	p.l.Debug("Get all records")

	cur := r.URL.Query().Get("currency")
	lp, err := p.productDB.GetProducts(cur)

	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		data.ToJSON(&GenericError{Message: err.Error()}, rw)
		return
	}

	err = data.ToJSON(lp, rw)
	if err != nil {
		p.l.Error("Unable to serialize product", "error", err)
		return
	}
}

func (p *Products) ListSingle(rw http.ResponseWriter, r *http.Request) {
	id := getProductID(r)
	cur := r.URL.Query().Get("currency")

	p.l.Debug("Get record", "id", id)

	prod, err := p.productDB.GetProductByID(id, cur)

	switch err {
	case nil:

	case data.ErrProductNotFound:
		p.l.Error("Fetching product", "error", err)

		rw.WriteHeader(http.StatusNotFound)
		data.ToJSON(&GenericError{Message: err.Error()}, rw)
		return
	default:
		p.l.Error("Fetching product", "error", err)

		rw.WriteHeader(http.StatusInternalServerError)
		data.ToJSON(&GenericError{Message: err.Error()}, rw)
		return
	}

	err = data.ToJSON(prod, rw)
	if err != nil {
		// we should never be here but log the error just incase
		p.l.Error("Serializing product", "error", err)
	}
}
