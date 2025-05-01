package data

import (
	"context"
	"fmt"

	protos "github.com/dolmatovDan/gRPC/currency"
	"github.com/go-playground/validator"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	SKU         string  `json:"sku"`
	CreatedOn   string  `json:"-"`
	UpdatedOn   string  `json:"-"`
	DeletedOn   string  `json:"-"`
}

type Products []*Product

type ProductsDB struct {
	currency protos.CurrencyClient
	log      hclog.Logger
	rates    map[string]float64
	client   protos.Currency_SubscribeRatesClient
	db       *gorm.DB
}

func NewProductsDB(c protos.CurrencyClient, l hclog.Logger) *ProductsDB {
	pb := &ProductsDB{c, l, make(map[string]float64), nil, nil}

	go pb.handleUpdates()

	pb.initDB()
	return pb
}

func (p *ProductsDB) initDB() {
	dsn := "host=localhost user=postgres password=yourpassword dbname=postgres port=5433 sslmode=disable"
	var err error
	p.db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		p.log.Error("Unable to connect to data base", "err", err)
		return
	}

	p.db.AutoMigrate(&Product{})
}

func (p *ProductsDB) handleUpdates() {
	sub, err := p.currency.SubscribeRates(context.Background())
	if err != nil {
		p.log.Error("Unable to subscibe for rates", "error", err)
		return
	}

	p.client = sub

	for {
		rr, err := sub.Recv()
		if grpcError := rr.GetError(); grpcError != nil {
			p.log.Error("Error subscribing for rates", "error", grpcError)
			continue
		}

		if resp := rr.GetRateResponse(); resp != nil {
			p.log.Info("Received updated rate from server", "dest", resp.GetDestination().String())

			if err != nil {
				p.log.Error("Error receiveing message", "error", err)
				return
			}

			p.rates[resp.GetDestination().String()] = resp.Rate
		}

	}
}

func (p *ProductsDB) GetProducts(currency string) (Products, error) {
	var dbProducts []Product
	if err := p.db.Find(&dbProducts).Error; err != nil {
		return nil, err
	}
	var productList Products
	for _, p := range dbProducts {
		productList = append(productList, &p)
	}
	if currency == "" {
		return productList, nil
	}

	rate, err := p.getRate(currency)
	if err != nil {
		p.log.Error("Unable to get rate", "currency", currency, "error", err)
		return nil, err
	}

	pr := Products{}
	for _, p := range productList {
		np := *p
		np.Price = np.Price / rate
		pr = append(pr, &np)
	}

	return pr, nil
}

func (pdb *ProductsDB) AddProduct(p Product) {
	if err := pdb.db.Create(&p).Error; err != nil {
		pdb.log.Error("Can't create products", "err", err)
		return
	}
}

func (pdb *ProductsDB) UpdateProduct(p Product) error {
	productList, err := pdb.GetProducts("")
	if err != nil {
		pdb.log.Info("Can't connect to data base")
		return err
	}
	i := pdb.findIndexByProductID(p.ID)
	if i == -1 {
		return ErrProductNotFound
	}

	// update the product in the DB
	productList[i] = &p

	if err := pdb.db.Model(&Product{}).Where("id = ?", p.ID).Updates(&p).Error; err != nil {
		pdb.log.Error("Can't update product", "err", err)
		return err
	}

	return nil
}

func (p *Product) Validate() error {
	validate := validator.New()
	validate.RegisterValidation("sku", validateSKU)

	return validate.Struct(p)
}

var ErrProductNotFound = fmt.Errorf("Product not found")

func (pdb *ProductsDB) DeleteProduct(id int) error {
	if err := pdb.db.Delete(&Product{}, id).Error; err != nil {
		return ErrProductNotFound
	}

	return nil
}

func (p *ProductsDB) GetProductByID(id int, currency string) (*Product, error) {
	productList, err := p.GetProducts("")
	if err != nil {
		p.log.Info("Can't connect to data base")
		return nil, err
	}
	i := p.findIndexByProductID(id)
	if id == -1 {
		return nil, ErrProductNotFound
	}

	if currency == "" {
		return productList[i], nil
	}

	rate, err := p.getRate(currency)
	if err != nil {
		p.log.Error("Unable to get rate", "currency", currency, "error", err)
		return nil, err
	}

	np := *productList[i]
	np.Price *= rate

	return &np, nil
}

func (pdb *ProductsDB) findIndexByProductID(id int) int {
	productList, err := pdb.GetProducts("")
	if err != nil {
		pdb.log.Info("Can't connect to data base")
		return 0
	}
	for i, p := range productList {
		if p.ID == id {
			return i
		}
	}

	return -1
}

func (p *ProductsDB) getRate(dest string) (float64, error) {
	if r, ok := p.rates[dest]; ok {
		return r, nil
	}

	rr := &protos.RateRequest{
		Base:        protos.Currencies(protos.Currencies_value["RUB"]),
		Destination: protos.Currencies(protos.Currencies_value[dest]),
	}

	// initial rate
	resp, err := p.currency.GetRate(context.Background(), rr)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			md := s.Details()[0].(*protos.RateRequest)
			if s.Code() == codes.InvalidArgument {
				return -1, fmt.Errorf("Unable to get rate from currency server, destination and base currency can not be the same, base: %s, dest: %s", md.Base.String(), md.Destination.String())
			}
			return -1, fmt.Errorf("Unable to get rate from currency server, base: %s, dest: %s", md.Base.String(), md.Destination.String())
		}
	}

	p.rates[dest] = resp.Rate // update cache

	// subscribe
	p.client.Send(rr)

	return resp.Rate, err
}
