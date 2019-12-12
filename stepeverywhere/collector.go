package stepeverywhere

import (
	"context"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/glassechidna/awsctx/service/organizationsctx"
	"github.com/pkg/errors"
	"strings"
)

type Collector struct {
	orgs           organizationsctx.Organizations
	defaultRegions []string
}

func NewCollector(orgs organizationsctx.Organizations, defaultRegions []string) *Collector {
	defaultRegions = fixupRegions(defaultRegions)
	return &Collector{orgs: orgs, defaultRegions: defaultRegions}
}

func fixupRegions(defaultRegions []string) []string {
	var regions []string

	for _, region := range defaultRegions {
		if trimmed := strings.TrimSpace(region); len(trimmed) > 0 {
			regions = append(regions, trimmed)
		}
	}

	if len(regions) == 0 {
		for regionId := range endpoints.AwsPartition().Regions() {
			regions = append(regions, regionId)
		}
	}

	return regions
}

type CollectAccountIdsInput struct {
	Regions  []string
	RoleName string
}

type CollectAccountIdsOutput struct {
	Contexts []Context
}

func (c *Collector) CollectAccountIds(ctx context.Context, input *CollectAccountIdsInput) (*CollectAccountIdsOutput, error) {
	var accounts []*organizations.Account

	err := c.orgs.ListAccountsPagesWithContext(ctx, &organizations.ListAccountsInput{}, func(page *organizations.ListAccountsOutput, lastPage bool) bool {
		accounts = append(accounts, page.Accounts...)
		return !lastPage
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	regions := input.Regions
	if len(regions) == 0 {
		regions = c.defaultRegions
	}

	var contexts []Context
	for _, account := range accounts {
		if *account.Status != organizations.AccountStatusActive {
			continue
		}

		for _, region := range regions {
			contexts = append(contexts, Context{
				AccountId: *account.Id,
				Region:    region,
				RoleName:  input.RoleName,
			})
		}
	}

	return &CollectAccountIdsOutput{
		Contexts: contexts,
	}, nil
}
