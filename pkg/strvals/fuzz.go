package strvals


func Fuzz(data []byte) int {
	_, err := Parse(string(data))
	if err != nil {
		return 0
	}
	return 1
}
