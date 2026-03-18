package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog"
)

const testProductName = "MyProduct"

type mockMarketplaceClient struct {
	listEntitiesFunc   func(ctx context.Context, params *marketplacecatalog.ListEntitiesInput, optFns ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error)
	describeEntityFunc func(ctx context.Context, params *marketplacecatalog.DescribeEntityInput, optFns ...func(*marketplacecatalog.Options)) (*marketplacecatalog.DescribeEntityOutput, error)
	startChangeSetFunc func(ctx context.Context, params *marketplacecatalog.StartChangeSetInput, optFns ...func(*marketplacecatalog.Options)) (*marketplacecatalog.StartChangeSetOutput, error)
}

func (m *mockMarketplaceClient) ListEntities(ctx context.Context, params *marketplacecatalog.ListEntitiesInput, optFns ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
	return m.listEntitiesFunc(ctx, params, optFns...)
}

func (m *mockMarketplaceClient) DescribeEntity(ctx context.Context, params *marketplacecatalog.DescribeEntityInput, optFns ...func(*marketplacecatalog.Options)) (*marketplacecatalog.DescribeEntityOutput, error) {
	return m.describeEntityFunc(ctx, params, optFns...)
}

func (m *mockMarketplaceClient) StartChangeSet(ctx context.Context, params *marketplacecatalog.StartChangeSetInput, optFns ...func(*marketplacecatalog.Options)) (*marketplacecatalog.StartChangeSetOutput, error) {
	return m.startChangeSetFunc(ctx, params, optFns...)
}
