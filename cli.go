package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// loadReleaseNotes resolves release notes from either the inline string or a file path.
func loadReleaseNotes(notes, notesFile string) (string, error) {
	if notesFile != "" {
		data, err := os.ReadFile(notesFile) //nolint:gosec // G304: notesFile is a CLI flag value provided by the operator, intentional file read
		if err != nil {
			return "", fmt.Errorf("failed to read release notes file: %w", err)
		}
		return string(data), nil
	}
	if notes == "" {
		return "", errors.New("--release-notes or --release-notes-file is required")
	}
	return notes, nil
}

func dumpVersionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump-versions [product]",
		Short: "Dump marketplace catalog data for all product versions to the all-versions.yaml YAML file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return dumpVersions(args[0])
		},
	}
	return cmd
}

func addVersionCmd() *cobra.Command {
	var noOp bool
	cmd := &cobra.Command{
		Use:   "push-version [product] [version]",
		Short: "Push local state of the product version's YAML file into a new version",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return pushNewVersion(args[0], noOp, args[1])
		},
	}
	cmd.Flags().BoolVar(&noOp, "no-op", false, "Print the changeset JSON to stdout without creating the changeset")
	return cmd
}

func dumpProductCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump [product]",
		Short: "Dump marketplace catalog data for a product to a YAML file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return dumpProduct(args[0])
		},
	}
	return cmd
}

func listProductsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [product-type]",
		Short: "List all my AWS Marketplace products of a given type, or 'all' for all types",
		Long: `List AWS Marketplace products. You can specify a product type or 'all'.
Valid product types are:
  - ServerProduct
  - ContainerProduct
  - DataProduct
  - MachinelearningProduct
  - SaaSProduct
  - ServiceProduct
  - SolutionProduct
  - SupportProduct`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return listProducts(args[0])
		},
	}
	return cmd
}

func updateProductCmd() *cobra.Command {
	var noOp bool
	cmd := &cobra.Command{
		Use:   "update [product]",
		Short: "Update a product's information based on the data provided in its local YAML representation",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return updateProduct(args[0], noOp)
		},
	}
	cmd.Flags().BoolVar(&noOp, "no-op", false, "Print the changeset JSON to stdout without creating the changeset")
	return cmd
}

func cloneProductCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone [product] [src-version] [dst-version]",
		Short: "Copy the YAML data from the src version to the dst version",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			return cloneProductVersion(args[0], args[1], args[2])
		},
	}
	return cmd
}

func releaseCmd() *cobra.Command {
	var noOp bool
	var image, releaseNotes, releaseNotesFile, baseVersion string

	cmd := &cobra.Command{
		Use:   "release [product] [new-version]",
		Short: "Automated release: clone latest version, update image and release notes, push new version",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			if image == "" {
				return errors.New("--image is required")
			}
			notes, err := loadReleaseNotes(releaseNotes, releaseNotesFile)
			if err != nil {
				return err
			}
			return releaseVersion(args[0], args[1], image, notes, baseVersion, noOp)
		},
	}

	cmd.Flags().StringVar(&image, "image", "", "Docker image URI (required)")
	cmd.Flags().StringVar(&releaseNotes, "release-notes", "", "Release notes text")
	cmd.Flags().StringVar(&releaseNotesFile, "release-notes-file", "", "Path to file containing release notes")
	cmd.Flags().StringVar(&baseVersion, "base-version", "", "Base version to clone from (auto-detects latest if not specified)")
	cmd.Flags().BoolVar(&noOp, "no-op", false, "Print the changeset JSON to stdout without creating the changeset")
	return cmd
}

func mainFunc() {
	rootCmd := &cobra.Command{Use: "aws-marketplace-cli"}
	rootCmd.AddCommand(
		listProductsCmd(),
		dumpProductCmd(),
		updateProductCmd(),
		dumpVersionsCmd(),
		addVersionCmd(),
		cloneProductCmd(),
		releaseCmd(),
	)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
