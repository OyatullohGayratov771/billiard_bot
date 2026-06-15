package repository

import (
	"database/sql"

	"shop-service/internal/models"
)

type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

const cols = `id, name, description, price, category, emoji, image_url, in_stock, sort_order, created_at`

func scanAll(rows *sql.Rows) ([]*models.Product, error) {
	list := make([]*models.Product, 0)
	for rows.Next() {
		p := &models.Product{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Category,
			&p.Emoji, &p.ImageURL, &p.InStock, &p.SortOrder, &p.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// ListInStock — mijozlar uchun (faqat sotuvdagi).
func (r *Repo) ListInStock() ([]*models.Product, error) {
	rows, err := r.db.Query(`SELECT `+cols+` FROM products WHERE in_stock=true ORDER BY sort_order ASC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAll(rows)
}

// ListAll — admin uchun (barchasi).
func (r *Repo) ListAll() ([]*models.Product, error) {
	rows, err := r.db.Query(`SELECT ` + cols + ` FROM products ORDER BY sort_order ASC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAll(rows)
}

func (r *Repo) Create(p *models.Product) (int64, error) {
	var id int64
	err := r.db.QueryRow(`
		INSERT INTO products (name, description, price, category, emoji, image_url, in_stock, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		p.Name, p.Description, p.Price, p.Category, p.Emoji, p.ImageURL, p.InStock, p.SortOrder,
	).Scan(&id)
	return id, err
}

// Update — mavjud bo'lsa true qaytaradi.
func (r *Repo) Update(id int64, p *models.Product) (bool, error) {
	res, err := r.db.Exec(`
		UPDATE products SET name=$1, description=$2, price=$3, category=$4,
		       emoji=$5, image_url=$6, in_stock=$7, sort_order=$8 WHERE id=$9`,
		p.Name, p.Description, p.Price, p.Category, p.Emoji, p.ImageURL, p.InStock, p.SortOrder, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ImageURL — o'chirishdan oldin rasm yo'lini olish uchun.
func (r *Repo) ImageURL(id int64) string {
	var u string
	_ = r.db.QueryRow(`SELECT image_url FROM products WHERE id=$1`, id).Scan(&u)
	return u
}

func (r *Repo) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM products WHERE id=$1`, id)
	return err
}
