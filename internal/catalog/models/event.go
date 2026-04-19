package models

type EventRequest struct {
	ActorKey string `json:"actorKey"`
}

type EventResponse struct {
	Action   string  `json:"action"`
	Recorded bool    `json:"recorded"`
	Sticker  Sticker `json:"sticker"`
}
