package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func main() {
	rootCmd := &cobra.Command{Use: "aws-marketplace-cli"}
	rootCmd.AddCommand(dumpCmd(), updateDescriptionCmd(), listCmd())
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [product-type]",
		Short: "List all my AWS Marketplace products of a given type, such as ContainerProduct",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			productType := args[0]
			cfg, err := config.LoadDefaultConfig(context.Background())
			if err != nil {
				return err
			}

			//log.Printf("Entity type: %v", productType)

			svc := marketplacecatalog.NewFromConfig(cfg)
			params := &marketplacecatalog.ListEntitiesInput{
				Catalog:    aws.String("AWSMarketplace"),
				EntityType: aws.String(productType),
				MaxResults: aws.Int32(10),
			}
			for {
				resp, err := svc.ListEntities(context.Background(), params)
				if err != nil {
					return err
				}
				for _, entity := range resp.EntitySummaryList {
					fmt.Printf("%s\n", *entity.Name)
				}
				if resp.NextToken == nil {
					break
				}
				params.NextToken = resp.NextToken
			}
			return nil
		},
	}
	return cmd
}

func dumpCmd() *cobra.Command {
	var productName string

	cmd := &cobra.Command{
		Use:   "dump [product name]",
		Short: "Dump marketplace catalog data for a product to a YAML file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			productName = args[0]

			cfg, err := config.LoadDefaultConfig(context.Background())
			if err != nil {
				return err
			}

			svc := marketplacecatalog.NewFromConfig(cfg)

			entityID, err := getProductEntityID(svc, &productName)
			if err != nil {
				return err
			}

			resp, err := svc.DescribeEntity(context.Background(), &marketplacecatalog.DescribeEntityInput{
				EntityId: entityID,
				Catalog:  aws.String("AWSMarketplace"),
			})
			if err != nil {
				return err
			}

			var details EntityDetails
			if err := json.Unmarshal([]byte(*resp.Details), &details); err != nil {
				return err
			}

			fileName := getYamlFilePath(productName)
			data, err := yaml.Marshal(details)
			if err != nil {
				return err
			}

			// Check if file has changed before writing to it
			if _, err := os.Stat(fileName); err == nil {
				existingData, err := ioutil.ReadFile(fileName)
				if err != nil {
					return err
				}
				if bytes.Equal(existingData, data) {
					fmt.Printf("Data for entity %s has not changed %s \n", *entityID, data)
					return nil
				}
			}

			if err := ioutil.WriteFile(fileName, data, 0644); err != nil {
				return err
			}

			fmt.Printf("Data written to %s\n", fileName)
			return nil
		},
	}

	return cmd
}

func getYamlFilePath(productName string) string {
	dirName := fmt.Sprintf("products/%s", productName)
	os.MkdirAll(dirName, 0755)
	return filepath.Join(dirName, "description.yaml")
}

func getProductEntityID(svc *marketplacecatalog.Client, productName *string) (*string, error) {

	input := &marketplacecatalog.ListEntitiesInput{
		Catalog:    aws.String("AWSMarketplace"),
		EntityType: aws.String("ContainerProduct"),
	}
	res, err := svc.ListEntities(context.Background(), input)
	if err != nil {
		return nil, err
	}
	if len(res.EntitySummaryList) == 0 {
		return nil, fmt.Errorf("could not find entity ID for product %s", *productName)
	}
	for _, entity := range res.EntitySummaryList {
		if *entity.Name == *productName {
			return entity.EntityId, nil
		}
	}

	return nil, errors.New("entity not found")
}

func updateDescriptionCmd() *cobra.Command {
	var noOp bool
	cmd := &cobra.Command{
		Use:   "update-description [product name]",
		Short: "Push a changeset created by a YAML file to the marketplace catalog API",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			productName := args[0]

			cfg, err := config.LoadDefaultConfig(context.Background())
			if err != nil {
				return errors.New("couldn't load default config")
			}

			svc := marketplacecatalog.NewFromConfig(cfg)

			entityID, err := getProductEntityID(svc, &productName)

			if err != nil {
				return err
			}

			// Read the YAML file
			data, err := ioutil.ReadFile(getYamlFilePath(productName))
			if err != nil {
				return err
			}
			var details EntityDetails
			if err := yaml.Unmarshal(data, &details); err != nil {
				return err
			}

			detailsBytes, err := json.Marshal(details.Description)
			if err != nil {
				return err
			}

			// Create a changeset to update the product
			change := types.Change{
				ChangeType: aws.String("UpdateInformation"),
				ChangeName: aws.String("UpdateProductInformation"),
				Entity: &types.Entity{
					Type:       aws.String("ContainerProduct@1.0"),
					Identifier: entityID,
				},
				Details: aws.String(string(detailsBytes)),
			}
			changeSetInput := &marketplacecatalog.StartChangeSetInput{
				Catalog: aws.String("AWSMarketplace"),
				ChangeSet: []types.Change{
					change,
				},
			}

			if noOp {
				changeSetJSON, _ := json.MarshalIndent(changeSetInput, "", "  ")
				fmt.Println(string(changeSetJSON))
				return nil
			}

			_, err = svc.StartChangeSet(context.TODO(), changeSetInput)

			if err != nil {
				return errors.New("could not start change set: " + err.Error())
			}

			fmt.Printf("Changeset created for product %s with entity ID %s\n", productName, *entityID)
			return nil
		},
	}

	cmd.Flags().BoolVar(&noOp, "no-op", false, "Print the changeset JSON to stdout without creating the changeset")
	return cmd
}
