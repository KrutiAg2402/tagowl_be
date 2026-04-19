package models

type HomeSection struct {
	Key      string    `json:"key"`
	Title    string    `json:"title"`
	Stickers []Sticker `json:"stickers"`
}

type HomeResponse struct {
	Categories []string      `json:"categories"`
	Sections   []HomeSection `json:"sections"`
}
