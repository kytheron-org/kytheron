package main

import (
	"errors"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/kytheron-org/kytheron/config"
	"github.com/spf13/cobra"
	"log"
	"os"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate KytheronDB schema",
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")

		cfg, err := config.Load(configPath)
		if err != nil {
			log.Fatal(err)
		}

		m, err := migrate.New("file://db/migrations", fmt.Sprintf("postgres://%s", cfg.Database.Url))
		if err != nil {
			log.Fatal(err)
		}
		err = m.Up()
		if err != nil && errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("No changes found")
			os.Exit(0)
		} else if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
