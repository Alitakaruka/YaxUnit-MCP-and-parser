package service

type Module struct {
	Name    string   `json:"module"`
	Methods []Method `json:"methods"`
}

type Method struct {
	Name        string      `json:"name"`
	Module      string      `json:"module"`
	Description string      `json:"description"`
	Parameters  []Parameter `json:"parameters"`
	Returns     *Return     `json:"returns,omitempty"`
	Examples    []string    `json:"examples,omitempty"`
}

type Parameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type,omitempty"`
	Description string      `json:"description,omitempty"`
	Values      []Parameter `json:"values,omitempty"`
}

type Return struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type PageData struct {
	URL    string
	Header string
	Data   string
	Module Module
}
