package actor

import "sync"

type FixedActor struct {
	CreatorID      uint
	CreatorLogin   string
	ModeratorID    uint
	ModeratorLogin string
}

var (
	once  sync.Once
	value FixedActor
)

func Current() FixedActor {
	once.Do(func() {
		value = FixedActor{
			CreatorID:      1,
			CreatorLogin:   "creator",
			ModeratorID:    2,
			ModeratorLogin: "moderator",
		}
	})

	return value
}
