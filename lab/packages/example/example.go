package example

type Difficulty uint

const (
	DifficultyEasy Difficulty = iota
	DifficultyMedium
	DifficultyHard
	difficultyMax
)

var difficultyNames = map[Difficulty]string{
	DifficultyEasy:   "Easy",
	DifficultyMedium: "Medium",
	DifficultyHard:   "Hard",
}

func IsValidDifficulty(d Difficulty) bool {
	return isEnumValid(d, difficultyMax)
}

func isEnumValid[T ~uint](v T, max T) bool {
	return v < max
}

func (d Difficulty) String() string {
	if !IsValidDifficulty(d) {
		return "Invalid"
	}
	return difficultyNames[d]
}
