package audio

type Audio interface {
	GetVolume() float64
	SetVolume(float64)
	Load(string) (int, error)
	Start()
	Stop()
	Play(id int) int
	Mute(bool)
	Speak(string) int
}
