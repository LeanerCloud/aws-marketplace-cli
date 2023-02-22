package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

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
					fmt.Printf("Product: %s \t EntityID: %s\n", *entity.Name, *entity.EntityId)
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
	var entityID string
	cmd := &cobra.Command{
		Use:   "dump [entity ID]",
		Short: "Dump marketplace catalog data for a product to a YAML file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			entityID = args[0]
			cfg, err := config.LoadDefaultConfig(context.Background())
			if err != nil {
				return err
			}
			svc := marketplacecatalog.NewFromConfig(cfg)
			resp, err := svc.DescribeEntity(context.TODO(), &marketplacecatalog.DescribeEntityInput{
				EntityId: &entityID,
				Catalog:  aws.String("AWSMarketplace"),
			})
			if err != nil {
				return err
			}

			var details EntityDetails
			if err := json.Unmarshal([]byte(*resp.Details), &details); err != nil {
				return err
			}

			filename := fmt.Sprintf("%s_%s.yaml", details.Description.ProductTitle, entityID)
			data, err := yaml.Marshal(details)
			if err != nil {
				return err
			}
			if err := ioutil.WriteFile(filename, data, 0644); err != nil {
				return err
			}
			fmt.Printf("Data written to %s\n", filename)
			return nil
		},
	}
	return cmd
}

type Entity map[string]interface{}

func updateDescriptionCmd() *cobra.Command {
	var printFlag bool
	cmd := &cobra.Command{
		Use:   "update-description [filename]",
		Short: "Create a changeset updating the product description information to match the data from the given YAML file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			filename := args[0]
			entityID := strings.Split(strings.TrimSuffix(filepath.Base(filename), ".yaml"), "_")[1]

			fmt.Println("Updating product")

			cfg, err := config.LoadDefaultConfig(context.Background())
			if err != nil {
				return err
			}
			svc := marketplacecatalog.NewFromConfig(cfg)
			data, err := ioutil.ReadFile(filename)
			if err != nil {
				return err
			}
			var pd Details
			if err := yaml.Unmarshal(data, &pd); err != nil {
				return err
			}
			detailsJSON, err := json.Marshal(pd.Description)
			if err != nil {
				return err
			}
			//detailsJSONString := fmt.Sprintf("%q", string(detailsJSON))
			detailsJSONString := string(detailsJSON)
			if printFlag {
				fmt.Println(detailsJSONString)
				return nil
			}

			changeSetInput := &marketplacecatalog.StartChangeSetInput{
				Catalog: aws.String("AWSMarketplace"),
				ChangeSet: []types.Change{
					{
						ChangeName: aws.String("UpdateProductInformation"),
						ChangeType: aws.String("UpdateInformation"),
						Entity: &types.Entity{
							Type:       aws.String("ContainerProduct@1.0"),
							Identifier: aws.String(entityID),
						},
						Details: aws.String(detailsJSONString),
					},
				},
			}
			_, err = svc.StartChangeSet(context.Background(), changeSetInput)
			if err != nil {
				return err
			}
			fmt.Println("Changeset created successfully")
			return nil
		},
	}
	cmd.Flags().BoolVar(&printFlag, "print", false, "Print escaped JSON string to stdout instead of creating a changeset")
	return cmd
}
