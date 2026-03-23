package entity

type User struct {
	ID          int
	Pseudo      string `json:"pseudo"`
	First_name  string `json:"first_name"`
	Last_name   string `json:"last_name"`
	Description string `json:"description"`
	ImageUrl    string `json:"image_url"`
}
