module github.com/fclairamb/gorm-migrate

go 1.15

require (
	gorm.io/driver/sqlite v1.1.4 // for tests only
	gorm.io/gorm v1.23.6
)

// replace gorm.io/driver/sqlite => /Users/florent/go/src/github.com/fclairamb/sqlite
