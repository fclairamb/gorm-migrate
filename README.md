![Build](https://github.com/fclairamb/gorm-migrate/workflows/Build/badge.svg)
[![codecov](https://codecov.io/gh/fclairamb/gorm-migrate/branch/main/graph/badge.svg)](https://codecov.io/gh/fclairamb/gorm-migrate)
[![Go Report Card](https://goreportcard.com/badge/fclairamb/gorm-migrate)](https://goreportcard.com/report/fclairamb/gorm-migrate)
![Go version](https://img.shields.io/github/go-mod/go-version/fclairamb/gorm-migrate.svg)
[![Go.Dev reference](https://img.shields.io/badge/go.dev-reference-blue?logo=go&logoColor=white)](https://pkg.go.dev/github.com/fclairamb/gorm-migrate?tab=doc)
[![MIT license](https://img.shields.io/badge/license-MIT-brightgreen.svg)](https://opensource.org/licenses/MIT)


# Gorm database migration

Simple library to take advantage of the [gorm's migration API](https://gorm.io/docs/migration.html).

## Choices

* It only applies migrations. It's up to you to chose when to apply each operation.
* Any failure cancels every single change (including the one to the migrations listing table)
* There are no consistency checks between migrations. ALl migrations will be applied as long as they are after
  the current migration.
  
## Known issues

* The column dropping operation doesn't work in SQLite because this library performs all the changes within a 
  transaction, and the SQLite Migrator's DropColumn works by creating a transaction in pure-SQL with a `BEGIN`
  when a sub-transaction using a `SAVEPOINT` is necessary here.

## How to use

```golang

import (
	"gorm.io/gorm"
	migrate "github.com/fclairamb/gorm-migrate"
)

func performMigrations(db *gorm.DB) error {
    
    steps := []*migrate.MigrationStep{
        {
            Name: "2020-09-12 01:00",
            Up: func(db *gorm.DB) error {
                return db.Migrator().AutoMigrate(&User{})
            },
            Down: func(db *gorm.DB) error {
                if db.Migrator().HasTable(&User{}) {
                    return db.Migrator().DropTable(&User{})
                }
                return nil
            },
        },
        {
            Name: "2020-09-12 02:00",
            Up: func(db *gorm.DB) error {
                return nil
            },
            Down: func(db *gorm.DB) error {
                return nil
            },
        },
    }
    
    if nb, err := migrate.Migrate(db, steps, migrate.UpFull); err != nil {
        return err
    } else if nb > 0 {
        log.Printf("Performed %d migrations !\n", nb)
    }
    return nil
}
```
