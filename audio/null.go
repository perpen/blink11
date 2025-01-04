package audio

func NewNull() (Audio, error) {
	return &Null{}, nil
}

type Null struct {
}

func (a *Null) Mute(b bool) {
}

func (a *Null) GetVolume() float64 {
	return 0
}

func (a *Null) SetVolume(float64) {
}

func (a *Null) Load(string) (int, error) {
	return 0, nil
}

func (a *Null) Start() {
}

func (a *Null) Stop() {
}

func (a *Null) Play(id int) int {
	return 0
}

func (a *Null) Speak(text string) int {
	return 0
}
