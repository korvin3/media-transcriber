package domain

// WhisperModelOption describes one downloadable whisper.cpp model preset.
type WhisperModelOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	FileName    string `json:"fileName"`
	URL         string `json:"url"`
	SizeLabel   string `json:"sizeLabel,omitempty"`
	Description string `json:"description,omitempty"`
	Downloaded  bool   `json:"downloaded"`
	LocalPath   string `json:"localPath,omitempty"`
}
