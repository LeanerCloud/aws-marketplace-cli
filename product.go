package main

type EntityDetails struct {
	Description Description `json:"Description"`

	// These following fields may be exposed in a future version.

	// Dimensions  []struct {
	// 	Description string   `json:"Description"`
	// 	Key         string   `json:"Key"`
	// 	Name        string   `json:"Name"`
	// 	Types       []string `json:"Types"`
	// 	Unit        string   `json:"Unit"`
	// } `json:"Dimensions"`
	// PromotionalResources struct {
	// 	AdditionalResources []struct {
	// 		Text string `json:"Text"`
	// 		Type string `json:"Type"`
	// 		URL  string `json:"Url"`
	// 	} `json:"AdditionalResources"`
	// 	LogoURL          string `json:"LogoUrl"`
	// 	PromotionalMedia any    `json:"PromotionalMedia"`
	// 	Videos           []struct {
	// 		Title string `json:"Title"`
	// 		Type  string `json:"Type"`
	// 		URL   string `json:"Url"`
	// 	} `json:"Videos"`
	// } `json:"PromotionalResources"`
	// RegionAvailability struct {
	// 	FutureRegionSupport any      `json:"FutureRegionSupport"`
	// 	Regions             []string `json:"Regions"`
	// 	Restrict            []any    `json:"Restrict"`
	// } `json:"RegionAvailability"`
	// Repositories []struct {
	// 	Type string `json:"Type"`
	// 	URL  string `json:"Url"`
	// } `json:"Repositories"`
	// SupportInformation struct {
	// 	Description string `json:"Description"`
	// 	Resources   []any  `json:"Resources"`
	// } `json:"SupportInformation"`
	// Targeting struct {
	// 	PositiveTargeting struct {
	// 		BuyerAccounts []string `json:"BuyerAccounts"`
	// 	} `json:"PositiveTargeting"`
	// } `json:"Targeting"`
	// Versions []struct {
	// 	CreationDate    time.Time `json:"CreationDate"`
	// 	DeliveryOptions []struct {
	// 		Compatibility struct {
	// 			AwsServices []string `json:"AWSServices"`
	// 		} `json:"Compatibility"`
	// 		ID           string `json:"Id"`
	// 		Instructions struct {
	// 			Usage string `json:"Usage"`
	// 		} `json:"Instructions"`
	// 		Recommendations struct {
	// 			DeploymentResources []struct {
	// 				Text string `json:"Text"`
	// 				URL  string `json:"Url"`
	// 			} `json:"DeploymentResources"`
	// 		} `json:"Recommendations"`
	// 		ShortDescription string `json:"ShortDescription"`
	// 		SourceID         string `json:"SourceId"`
	// 		Title            string `json:"Title"`
	// 		Type             string `json:"Type"`
	// 		Visibility       string `json:"Visibility"`
	// 		IsRecommended    bool   `json:"isRecommended"`
	// 	} `json:"DeliveryOptions"`
	// 	ID           string `json:"Id"`
	// 	ReleaseNotes string `json:"ReleaseNotes"`
	// 	Sources      []struct {
	// 		Compatibility struct {
	// 			Platform string `json:"Platform"`
	// 		} `json:"Compatibility"`
	// 		ID     string   `json:"Id"`
	// 		Images []string `json:"Images"`
	// 		Type   string   `json:"Type"`
	// 	} `json:"Sources"`
	// 	UpgradeInstructions string `json:"UpgradeInstructions"`
	// 	VersionTitle        string `json:"VersionTitle"`
	// } `json:"Versions"`
}

// Seems to be a somehow flattened version of the Product Details
// that's missing the version information

type Description struct {
	Categories       []string `json:"Categories"`
	Highlights       []string `json:"Highlights"`
	LongDescription  string   `json:"LongDescription"`
	ProductTitle     string   `json:"ProductTitle"`
	SearchKeywords   []string `json:"SearchKeywords"`
	ShortDescription string   `json:"ShortDescription"`
}

type Details struct {
	Description Description
}
