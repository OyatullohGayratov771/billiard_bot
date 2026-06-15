package models

import "time"

// Product — do'kon mahsuloti.
type Product struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       int64     `json:"price"`
	Category    string    `json:"category"`
	Emoji       string    `json:"emoji"`
	ImageURL    string    `json:"image_url"`
	InStock     bool      `json:"in_stock"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
}
