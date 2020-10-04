package migrate_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	migrate "github.com/fclairamb/gorm-migrate"
)

func getDB(t *testing.T) *gorm.DB {
	filename := fmt.Sprintf("%s_%s.db", t.Name(), time.Now().Format("2006-01-02_15:04:05"))
	db, err := gorm.Open(
		sqlite.Open(filename),
		&gorm.Config{},
	)

	if err != nil {
		t.Fatalf("Couldn't instantiate DB: %v", err)
	}

	return db
}

var ErrBadMigration = errors.New("this is bad migration")

type User struct {
	gorm.Model
	FirstName string
	LastName  string
	Age       int
}

type Friend struct {
	gorm.Model
}

func TestMigrate(t *testing.T) {
	steps := []*migrate.MigrationStep{
		{
			Name: "000",
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
			Name: "001",
			Up: func(db *gorm.DB) error {
				return nil
			},
			Down: func(db *gorm.DB) error {
				if db.Migrator().HasTable(&Friend{}) {
					return db.Migrator().DropTable(&Friend{})
				}
				return nil
			},
		},
		{
			Name: "002",
			Up: func(db *gorm.DB) error {
				return nil
			},
			Down: func(db *gorm.DB) error {
				return nil
			},
		},
	}
	db := getDB(t)

	if nb, err := migrate.Migrate(db, steps[0:1], migrate.UpFull); err != nil {
		t.Fatalf("Couldn't migrate: %v", err)
	} else if nb != 1 {
		t.Fatalf("Wrong number of applied migrations: %d", nb)
	}

	if nb, err := migrate.Migrate(db, steps, migrate.UpFull); err != nil {
		t.Fatalf("Couldn't migrate: %v", err)
	} else if nb != 2 {
		t.Fatalf("Wrong number of applied migrations: %d", nb)
	}

	if nb, err := migrate.Migrate(db, steps, migrate.UpFull); err != nil {
		t.Fatalf("Couldn't migrate: %v", err)
	} else if nb != 0 {
		t.Fatalf("Wrong number of applied migrations: %d", nb)
	}

	if _, err := migrate.Migrate(db, steps, migrate.DownFull); err != nil {
		t.Fatalf("Couldn't migrate: %v", err)
	}
}

func TestMigrateError(t *testing.T) {
	steps := []*migrate.MigrationStep{
		{
			Name: "000",
			Up: func(db *gorm.DB) error {
				return nil
			},
			Down: func(db *gorm.DB) error {
				return nil
			},
		},
		{
			Name: "001",
			Up: func(db *gorm.DB) error {
				return ErrBadMigration
			},
			Down: func(db *gorm.DB) error {
				return ErrBadMigration
			},
		},
	}

	db := getDB(t)

	if nb, err := migrate.Migrate(db, steps, migrate.UpFull); err == nil || nb != 1 {
		t.Fatal(fmt.Sprintf("We should have failed with %d migration applied.", nb))
	}
}

type gormMigrations struct {
	Name  int
	Bogus string `gorm:"not null"`
}

func TestWreckedMigrationTable1(t *testing.T) {
	passNb := 0
	steps := []*migrate.MigrationStep{
		{
			Name: "000",
			Up: func(db *gorm.DB) error {
				passNb++
				if passNb == 1 {
					return db.Migrator().DropColumn(&gormMigrations{}, "name")
				} else if passNb == 2 {
					return db.Migrator().AutoMigrate(&gormMigrations{})
				}
				return nil
			},
			Down: func(db *gorm.DB) error {
				return nil
			},
		},
	}

	db := getDB(t)

	// First pass is a failure (lost one column)
	if _, err := migrate.Migrate(db, steps, migrate.UpFull); err == nil {
		t.Fatal("We should have failed")
	} else {
		db.Logger.Warn(context.Background(), "Err: %v", err)
	}
	// ==> NOTHING should have been done here (not even creating the migrations table)

	// Second pass is a failure (added a bogus column)
	if _, err := migrate.Migrate(db, steps, migrate.UpFull); err == nil {
		t.Fatal("We should have failed")
	} else {
		db.Logger.Warn(context.Background(), "Err: %v", err)
	}

	// Third pass is good
	if nb, err := migrate.Migrate(db, steps, migrate.UpFull); err != nil || nb != 1 {
		t.Fatalf("We should not have failed: %v (%d)", err, nb)
	}
}

func TestWreckedMigrationTable2(t *testing.T) {
	db := getDB(t)
	if err := db.Migrator().AutoMigrate(&gormMigrations{}); err != nil {
		t.Fatalf("Couldn't do the initial wrechking: %v", err)
	}

	if _, err := migrate.Migrate(db, []*migrate.MigrationStep{
		{Name: "000", Up: func(db *gorm.DB) error { return nil }},
	}, migrate.UpFull); err == nil {
		t.Fatal("We should have failed")
	}
}

func TestMissingUp(t *testing.T) {
	steps := []*migrate.MigrationStep{
		{
			Name: "000",
		},
	}

	db := getDB(t)

	// Up migration is missing
	if _, err := migrate.Migrate(db, steps, migrate.UpFull); err == nil {
		t.Fatal("We should have failed")
	} else if s := err.Error(); s != "bad migration: invalid migration 000: "+migrate.StepIssueUpMissing {
		t.Fatal(s)
	}
}

func TestMissingDown(t *testing.T) {
	steps := []*migrate.MigrationStep{
		{
			Name: "000",
			Up:   func(db *gorm.DB) error { return nil },
		},
	}

	db := getDB(t)

	// Up migration is missing
	if _, err := migrate.Migrate(db, steps, migrate.UpFull); err == nil {
		t.Fatal("We should have failed")
	} else if s := err.Error(); s != "bad migration: invalid migration 000: "+migrate.StepIssueDownMissing {
		t.Fatal(s)
	}
}

func TestBadlyOrderedMigrations(t *testing.T) {
	steps := []*migrate.MigrationStep{
		{
			Name: "001",
			Up:   func(db *gorm.DB) error { return nil },
			Down: func(db *gorm.DB) error { return nil },
		},
		{
			Name: "000",
			Up:   func(db *gorm.DB) error { return nil },
			Down: func(db *gorm.DB) error { return nil },
		},
	}

	db := getDB(t)

	// First pass is a failure
	if _, err := migrate.Migrate(db, steps, migrate.UpFull); err == nil {
		t.Fatal("We should have failed")
	} else if s := err.Error(); s != "bad migration: invalid migration 000: badly_ordered" {
		t.Fatal(s)
	}
}

func TestBadDirection(t *testing.T) {
	steps := []*migrate.MigrationStep{
		{
			Name: "000",
			Up:   func(db *gorm.DB) error { return nil },
			Down: func(db *gorm.DB) error { return nil },
		},
		{
			Name: "001",
			Up:   func(db *gorm.DB) error { return nil },
			Down: func(db *gorm.DB) error { return nil },
		},
	}

	db := getDB(t)

	// First pass is a failure
	if _, err := migrate.Migrate(db, steps, 0); err == nil || !errors.Is(err, migrate.ErrBadDirection) {
		t.Fatal("We should have failed")
	}
}

func TestDropColumn(t *testing.T) {
	steps := []*migrate.MigrationStep{
		{
			Name: "000",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&User{})
			},
			Down: func(db *gorm.DB) error { return nil },
		},
		{
			Name: "001",
			Up: func(db *gorm.DB) error {
				mig := db.Migrator()
				if err := mig.RenameColumn(&User{}, "first_name", "first_name_2"); err != nil {
					return err
				}
				return nil
			},
			Down: func(db *gorm.DB) error { return nil },
		},
		{
			Name: "002",
			Up: func(db *gorm.DB) error {
				mig := db.Migrator()
				// The DropColumn directive doesn't work because the reconstruction doesn't make use
				// of the gorm transactions properly.
				return mig.DropColumn(&User{}, "last_name")
			},
			Down: func(db *gorm.DB) error { return nil },
		},
	}

	db := getDB(t)

	if _, err := migrate.Migrate(db, steps, migrate.UpFull); err != nil {
		t.Fatalf("Problem: %v", err)
	}
}

func TestAlterColumn(t *testing.T) {
	steps := []*migrate.MigrationStep{
		{
			Name: "000",
			Up: func(db *gorm.DB) error {
				mig := db.Migrator()
				if err := db.AutoMigrate(&User{}); err != nil {
					return err
				}
				if err := mig.DropColumn(&User{}, "Age"); err != nil {
					return err
				}
				if err := mig.RenameColumn(&User{}, "FirstName", "Age"); err != nil {
					return err
				}
				return nil
			},
			Down: func(db *gorm.DB) error { return nil },
		},
		{
			Name: "001",
			Up: func(db *gorm.DB) error {
				mig := db.Migrator()
				if err := mig.AlterColumn(&User{}, "age"); err != nil {
					return err
				}
				return nil
			},
			Down: func(db *gorm.DB) error { return nil },
		},
	}

	db := getDB(t)

	if _, err := migrate.Migrate(db, steps, migrate.UpFull); err != nil {
		t.Fatalf("Problem: %v", err)
	}
}

func TestStepByStep(t *testing.T) {
	steps := []*migrate.MigrationStep{
		{
			Name: "000",
			Up:   func(db *gorm.DB) error { return nil },
			Down: func(db *gorm.DB) error { return nil },
		},
		{
			Name: "001",
			Up:   func(db *gorm.DB) error { return nil },
			Down: func(db *gorm.DB) error { return nil },
		},
		{
			Name: "002",
			Up:   func(db *gorm.DB) error { return nil },
			Down: func(db *gorm.DB) error { return nil },
		},
	}

	db := getDB(t)

	nbUps := 0

	for {
		if nb, err := migrate.Migrate(db, steps, migrate.UpOne); err != nil {
			t.Fatalf("Error: %v", err)
		} else {
			if nb != 1 {
				break
			}
			nbUps += nb
		}
	}

	if nbUps != 3 {
		t.Fatalf("Wrong nuimber of ups: %d", nbUps)
	}

	for {
		if nb, err := migrate.Migrate(db, steps, migrate.DownOne); err != nil {
			t.Fatalf("Error: %v", err)
		} else {
			if nb != 1 {
				break
			}
			nbUps -= nb
		}
	}

	if nbUps != 0 {
		t.Fatalf("Wrong number of downs: %d", nbUps)
	}
}
