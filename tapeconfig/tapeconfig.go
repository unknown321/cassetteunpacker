package tapeconfig

import (
	"fmt"
	"os"
	"path"
)

type TapeConfig struct {
	Reel       string
	ArtistX    float32
	ArtistY    float32
	TitleX     float32
	TitleY     float32
	ReelX      float32
	ReelY      float32
	TitleWidth float32
	TextColor  string
}

func (t *TapeConfig) ToString() string {
	format := `reel: %s
artistx: %.1f
artisty: %.1f
titlex: %.1f
titley: %.1f
reelx: %.1f
reely: %.1f
titlewidth: %.1f
textcolor: %s
`
	return fmt.Sprintf(format, t.Reel, t.ArtistX, t.ArtistY, t.TitleX, t.TitleY, t.ReelX, t.ReelY, t.TitleWidth, t.TextColor)
}

func (t *TapeConfig) Write(directory string) error {
	f, err := os.OpenFile(path.Join(directory, "config.txt"), os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("cannot open config file: %w", err)
	}

	if _, err = f.Write([]byte(t.ToString())); err != nil {
		return fmt.Errorf("cannot write config file: %w", err)
	}

	if err = f.Close(); err != nil {
		return fmt.Errorf("cannot close config file: %w", err)
	}

	return nil
}
