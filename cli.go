package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func dumpVersionsCmd() *cobra.Command {
	var productName string

	cmd := &cobra.Command{
		Use:   "dump-versions [product]",
		Short: "Dump marketplace catalog data for all product versions to the all-versions.yaml YAML file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			productName = args[0]
			return dumpVersions(productName)
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
			productName := args[0]
			version := args[1]
			return pushNewVersion(productName, noOp, version)
		},
	}

	cmd.Flags().BoolVar(&noOp, "no-op", false, "Print the changeset JSON to stdout without creating the changeset")
	return cmd
}

func dumpProductCmd() *cobra.Command {
	var productName string

	cmd := &cobra.Command{
		Use:   "dump [product]",
		Short: "Dump marketplace catalog data for a product to a YAML file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			productName = args[0]
			return dumpProduct(productName)
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
			productType := args[0]
			return listProducts(productType)
		},
	}
	return cmd
}

func updateProductCmd() *cobra.Command {
	var productName string
	var noOp bool

	cmd := &cobra.Command{
		Use:   "update [product]",
		Short: "Update a product's information based on the data provided in its local YAML representation",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			productName = args[0]
			return updateProduct(productName, noOp)
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
			productName, srcVersion, dstVersion := args[0], args[1], args[2]
			return cloneProductVersion(productName, srcVersion, dstVersion)
		},
	}
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
	)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
