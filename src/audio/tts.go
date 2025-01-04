package audio

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Gets an mp3 from the translate api and stores it in given path, in raw format
func ttsApi(lang, text, outPath string, volumeFactor, speedFactor float64) error {
	if _, err := os.Stat(outPath); err == nil {
		return nil
	}
	mp3Path := strings.TrimSuffix(outPath, ".raw") + ".mp3"
	mp3Writer, err := os.Create(mp3Path)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://translate.google.com/translate_tts?ie=UTF-8&total=1&idx=0&textlen=32&client=tw-ob&q=%s&tl=%s",
		url.QueryEscape(text), lang)
	response, err := http.Get(url)
	if err != nil {
		os.Remove(mp3Path)
		return err
	}
	defer response.Body.Close()
	status := response.StatusCode
	if status != 200 {
		return fmt.Errorf("translate_tts gave status %d", status)
	}
	_, err = io.Copy(mp3Writer, response.Body)
	mp3Writer.Close()
	if err != nil {
		os.Remove(mp3Path)
		return err
	}

	cmd := exec.Command("./scripts/audio-convert",
		strconv.FormatFloat(volumeFactor, 'f', -1, 64) ,
		strconv.FormatFloat(speedFactor, 'f', -1, 64) ,
		mp3Path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	err = cmd.Run()
	if err != nil {
		log.Fatalf(`audio-convert error: %+v`, err)
	}
	err = os.Remove(mp3Path)
	// if err != nil {
	// 	log.Fatalf(`Remove %s: %+v`, mp3Path, err)
	// }
	return nil
}
