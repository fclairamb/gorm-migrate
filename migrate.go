// Package migrate allows to easily manage database migration with gorm v2
package migrate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MigrationMethod declares the method we want to apply to the database.
type MigrationMethod func(db *gorm.DB) error

// MigrationStep declares a migration.
type MigrationStep struct {
	Name string          // Name is the name of the migration
	Up   MigrationMethod // Up is the upgrade migration method
	Down MigrationMethod // Down is the downgrade migration method
}

// Migrations contains the migration steps we want to apply.
type Migrations []*MigrationStep

type stepSave struct {
	ID            uint       `gorm:"primarykey"`
	Name          string     `gorm:"unique"` // Name of the migration
	MigrationTime *time.Time // Time of when the migration was applied
}

// ErrBadMigration is reported when a migration has a bad definition.
type ErrBadMigration struct {
	Name string // Name of the migration that has an issue
	Type string // Type of the issue
}

const (
	// StepIssueUpMissing means the issue is a missing upgrade method.
	StepIssueUpMissing = "up_missing"

	// StepIssueDownMissing means the issue is a missing downgrade method.
	StepIssueDownMissing = "down_missing"

	// StepIssueBadlyOrdered means the issue is badly ordered.
	StepIssueBadlyOrdered = "badly_ordered"
)

func (e *ErrBadMigration) Error() string {
	return fmt.Sprintf("invalid migration %s: %s", e.Name, e.Type)
}

func (stepSave) TableName() string {
	return "gorm_migrations"
}

var (
	// ErrBadDirection is returned when the direction is equal to 0.
	ErrBadDirection = fmt.Errorf("bad direction")

	// ErrInconsistentSteps is returned when we couldn't apply all the migration steps ups & downs
	ErrInconsistentSteps = fmt.Errorf("inconsistent steps")
)

func getLastAppliedMigration(db *gorm.DB) (*stepSave, error) {
	var step stepSave
	err := db.Order("name desc").Where("migration_time is not null").First(&step).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = nil
		}

		return nil, err
	}

	return &step, err
}

func getMigration(db *gorm.DB, name string) (*stepSave, error) {
	stepSave := &stepSave{Name: name}
	err := db.Where(stepSave).First(stepSave).Error

	if err == nil {
		return stepSave, nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}

	return nil, err
}

func saveMigration(db *gorm.DB, step *stepSave) error {
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"migration_time"}),
	}).Create(step).Error
}

func checkMigrations(steps Migrations) error {
	name := ""
	for _, s := range steps {
		if s.Name > name || name == "" {
			name = s.Name
		} else {
			return &ErrBadMigration{Name: s.Name, Type: StepIssueBadlyOrdered}
		}

		if s.Up == nil {
			return &ErrBadMigration{Name: s.Name, Type: StepIssueUpMissing}
		} else if s.Down == nil {
			return &ErrBadMigration{Name: s.Name, Type: StepIssueDownMissing}
		}
	}

	return nil
}

func getIndexForName(steps Migrations, name string) int {
	for k, v := range steps {
		if v.Name == name {
			return k
		}
	}

	return -1
}

func getSteps(steps Migrations, currentMigrationName string, direction int) Migrations {
	nextSteps := make([]*MigrationStep, 0)
	l := len(steps)
	var index int

	if currentMigrationName == "" {
		index = 0
	} else {
		index = getIndexForName(steps, currentMigrationName)
		if direction > 0 {
			index++
		}
	}

Loop:
	for {
		if direction == 0 || index == l {
			break
		}

		step := steps[index]
		nextSteps = append(nextSteps, step)

		switch {
		case direction > 0:
			{
				direction--
				index++
			}
		case direction < 0 && index > 0:
			{
				direction++
				index--
			}
		default:
			break Loop
		}
	}

	return nextSteps
}

// Migrate handles all the step of the migration steps.
func Migrate(db *gorm.DB, steps Migrations, direction int) (int, error) {
	nbApplied := 0

	return nbApplied, db.Transaction(func(db *gorm.DB) error {
		if err := checkMigrations(steps); err != nil {
			return fmt.Errorf("bad migration: %w", err)
		}
		if direction == 0 {
			return ErrBadDirection
		}
		lastMigrationName := ""
		if err := prepareMigrationTables(db); err != nil {
			return err
		}
		if lastMigration, err := getLastAppliedMigration(db); err != nil {
			return err
		} else if lastMigration != nil {
			lastMigrationName = lastMigration.Name
		}

		// If there's no applied migration and we're searching for downgrades, we can stop here
		if lastMigrationName == "" && direction < 0 {
			return nil
		}

		steps = getSteps(steps, lastMigrationName, direction)

		nb, err := applyMigration(db, steps, direction > 0)
		nbApplied = nb
		if err != nil {
			return fmt.Errorf("couldn't apply migrations: %w", err)
		}
		return nil
	})
}

// ValidateSteps validates that all the steps can be applied up & down
func ValidateSteps(db *gorm.DB, steps Migrations) error {
	db = db.Begin()
	defer db.Rollback()
	for pass := 1; pass <= 2; pass++ {
		db.Logger.Info(
			context.Background(),
			"Validation: Pass %d",
			pass,
		)
		nbUps := 0
		nbDowns := 0

		for _, direction := range []int{UpOne, DownOne} {
			for {
				db.Logger.Info(
					context.Background(),
					"Validation: Migrate direction=%d",
					direction,
				)
				if nb, err := Migrate(db, steps, direction); err != nil {
					return err
				} else if nb == 0 {
					break
				}
				if direction == UpOne {
					nbUps += 1
				} else if direction == DownOne {
					nbDowns += 1
				}
			}
		}

		if nbUps != nbDowns {
			return ErrInconsistentSteps
		}
	}
	return nil
}

func applyMigration(db *gorm.DB, steps Migrations, up bool) (int, error) {
	nb := 0

	for _, step := range steps {
		dbStep, err := getMigration(db, step.Name)
		if err != nil {
			return nb, err
		}

		if dbStep == nil {
			dbStep = &stepSave{Name: step.Name}
		}

		var method MigrationMethod
		var migrationTime *time.Time = nil

		direction := ""
		if up {
			direction = "upgrade"
			t := time.Now().UTC()
			migrationTime = &t
			method = step.Up
		} else {
			direction = "downgrade"
			method = step.Down
		}

		db.Logger.Warn(
			context.Background(),
			"Applying %s migration %s",
			direction,
			dbStep.Name,
		)

		if err := method(db); err != nil {
			return nb, fmt.Errorf("couldn't apply migration %s: %w", step.Name, err)
		}

		dbStep.MigrationTime = migrationTime
		if err := saveMigration(db, dbStep); err != nil {
			return nb, fmt.Errorf("couldn't save migration %s application: %w", step.Name, err)
		}
		nb++
	}

	return nb, nil
}

const (
	// UpFull is a complete upgrade migration.
	UpFull = 100000
	// DownFull is a complete downgrade migration.
	DownFull = -100000
	// UpOne is a single upgrade migration.
	UpOne = 1
	// DownOne is a single downgrade migration.
	DownOne = -1
)

func prepareMigrationTables(db *gorm.DB) error {
	return db.AutoMigrate(&stepSave{})
}
