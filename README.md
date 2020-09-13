![Build](https://github.com/fclairamb/gorm-migrate/workflows/Build/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/fclairamb/gorm-migrate)](https://goreportcard.com/report/fclairamb/gorm-migrate)
[![GoDoc](https://godoc.org/github.com/fclairamb/gorm-migrate?status.svg)](https://godoc.org/github.com/fclairamb/gorm-migrate)


# Gorm database migration

Simplistic library take advantage of the [gorm's migration API](https://gorm.io/docs/migration.html).

## Choices

* It only applies migrations. It's up to you to chose when to apply each operation.
* Any failure cancels every single change (including the one to migrations listing table)
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
