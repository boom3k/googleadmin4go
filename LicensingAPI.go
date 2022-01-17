package googleadmin4go

import (
	"context"
	"google.golang.org/api/licensing/v1"
	"google.golang.org/api/option"
	"log"
	"net/http"
	"strings"
)

var AllProducts = []*Product{
	&GoogleWorkspaceBusinessStarter,
	&GoogleWorkspaceBusinessStandard,
	&GoogleWorkspaceBusinessPlus,
	&GoogleWorkspaceEnterpriseEssentials,
	&GoogleWorkspaceEnterpriseStandard,
	&GoogleWorkspaceEnterprisePlus,
	&GoogleWorkspaceEssentials,
	&GoogleWorkspaceFrontline,
	&GoogleVault,
	&GoogleVaultFormerEmployee,
	&GoogleWorkspaceEnterprisePlusArchivedUser,
	&GSuiteBusinessArchivedUser,
	&WorkspaceBusinessPlusArchivedUser,
	&GoogleWorkspaceEnterpriseStandardArchivedUser,
}

func BuildNewLicensingAPI(client *http.Client, adminEmail string, ctx *context.Context) *LicensingAPI {
	var newLicensingAPI = &LicensingAPI{}
	return newLicensingAPI.Build(client, adminEmail, ctx)
}

func (receiver *LicensingAPI) Build(client *http.Client, adminEmail string, ctx *context.Context) *LicensingAPI {
	service, err := licensing.NewService(*ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}
	receiver.Service = service
	receiver.AdminEmail = adminEmail
	receiver.Domain = strings.Split(adminEmail, "@")[1]
	log.Printf("LicensingAPI <%s> as [%s]initialized...\n", receiver, adminEmail)
	return receiver
}

type LicensingAPI struct {
	Service    *licensing.Service
	AdminEmail string
	Domain     string
}

func (receiver *LicensingAPI) GetAllDomainLicenses(customerID string, products []*Product, maxResults int64) []*licensing.LicenseAssignment {
	var licenseAssignments []*licensing.LicenseAssignment
	for _, product := range products {
		log.Printf("Querying for <%s> licenses...\n", product.SKUName)
		currentSet := receiver.ListForProductAndSku(product.ProductID, product.SKUID, customerID, maxResults)
		if currentSet != nil {
			licenseAssignments = append(licenseAssignments, currentSet...)
		}
	}
	return licenseAssignments
}

func (receiver *LicensingAPI) GetAllDomainLicensesAsMap(customerID string, products []*Product, maxResults int64) map[Product][]*licensing.LicenseAssignment {
	productAssignmentsMap := make(map[Product][]*licensing.LicenseAssignment)
	for _, product := range products {
		log.Printf("Querying for <%s> licenses...\n", product.SKUName)
		currentSet := receiver.ListForProductAndSku(product.ProductID, product.SKUID, customerID, maxResults)
		productAssignmentsMap[*product] = currentSet
	}
	return productAssignmentsMap
}

func (receiver *LicensingAPI) Delete(product *Product, userID string) {
	_, err := receiver.Service.LicenseAssignments.Delete(product.ProductID, product.SKUID, userID).Do()
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}
}

func (receiver *LicensingAPI) Get(product *Product, userID string) *licensing.LicenseAssignment {
	response, err := receiver.Service.LicenseAssignments.Get(product.ProductID, product.SKUID, userID).Fields("*").Do()
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}
	return response
}

func (receiver *LicensingAPI) Insert(product *Product, userID string) *licensing.LicenseAssignment {
	licensingAssignmentInsert := &licensing.LicenseAssignmentInsert{}
	licensingAssignmentInsert.UserId = userID
	response, err := receiver.Service.LicenseAssignments.Insert(product.ProductID, product.SKUID, licensingAssignmentInsert).Do()
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}
	return response
}

func (receiver *LicensingAPI) ListForProduct(productID, customerID string, maxResults int64) []*licensing.LicenseAssignment {
	var licenseAssignments []*licensing.LicenseAssignment
	pageToken := ""
	skuName := ""
	request := receiver.Service.LicenseAssignments.ListForProduct(productID, customerID).Fields("*").MaxResults(maxResults)
	for {
		response, err := request.PageToken(pageToken).Do()
		if err != nil {
			if strings.Contains(err.Error(), "400") {
				log.Println(err.Error())
				return licenseAssignments
			} else {
				panic(err)
			}
		}
		skuName = response.Items[0].SkuName
		licenseAssignments = append(licenseAssignments, response.Items...)
		pageToken = response.NextPageToken
		if pageToken == "" {
			break
		}
		if response.Items == nil || len(response.Items) == 0 {
			log.Printf("{%s} - No further licenses under %s\n", customerID, productID)
			break
		}
		log.Printf("%s licenses thus far: %d\n", skuName, len(licenseAssignments))
	}
	log.Printf("%s licenses Total: %d\n", skuName, len(licenseAssignments))
	return licenseAssignments
}

func (receiver *LicensingAPI) ListForProductAndSku(productID, skuID, customerID string, maxResults int64) []*licensing.LicenseAssignment {
	var licenseAssignments []*licensing.LicenseAssignment
	pageToken := ""
	skuName := ""
	request := receiver.Service.LicenseAssignments.ListForProductAndSku(productID, skuID, customerID).Fields("*").MaxResults(maxResults)
	for {
		response, err := request.PageToken(pageToken).Do()
		if err != nil {
			if strings.Contains(err.Error(), "400") {
				log.Println(err.Error())
				return licenseAssignments
			} else {
				panic(err)
			}
		}
		if response.Items == nil || len(response.Items) == 0 {
			log.Printf("{%s} - No further licenses under %s -- %s\n", customerID, skuID, productID)
			break
		}
		skuName = response.Items[0].SkuName
		licenseAssignments = append(licenseAssignments, response.Items...)
		pageToken = response.NextPageToken
		if pageToken == "" {
			break
		}
		log.Printf("%s licenses thus far: %d\n", skuName, len(licenseAssignments))
	}
	log.Printf("%s licenses Total: %d\n", skuName, len(licenseAssignments))
	return licenseAssignments
}

func (receiver *LicensingAPI) Update(productID, skuID, userID string) *licensing.LicenseAssignment {
	newLicenseAssignment := &licensing.LicenseAssignment{
		ProductId: productID,
		SkuId:     skuID,
		UserId:    userID,
	}

	response, err := receiver.Service.LicenseAssignments.Update(productID, skuID, userID, newLicenseAssignment).Fields("*").Do()
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}
	return response
}

/*Licensing Product Custom Type*/
type Product struct {
	ProductID           string
	ProductName         string
	SKUID               string
	SKUName             string
	UnarchivalProductID string
	UnarchivalSKUID     string
}

func GetProductBySKUID(skuID string) *Product {
	for _, product := range AllProducts {
		if product.SKUID == skuID {
			return product
		}
	}
	return nil
}

func GetProductByName(skuName string) *Product {
	for _, product := range AllProducts {
		if product.SKUName == skuName {
			return product
		}
	}
	return nil
}

var GoogleWorkspaceBusinessStarter = Product{
	ProductID:   "Google-Apps",
	ProductName: "Google Workspace",
	SKUID:       "1010020027",
	SKUName:     "Google Workspace Business Starter",
}

var GoogleWorkspaceBusinessStandard = Product{
	ProductID:   "Google-Apps",
	ProductName: "Google Workspace",
	SKUID:       "1010020028",
	SKUName:     "Google Workspace Business Standard",
}

var GoogleWorkspaceBusinessPlus = Product{
	ProductID:   "Google-Apps",
	ProductName: "Google Workspace",
	SKUID:       "1010020025",
	SKUName:     "Google Workspace Business Plus",
}

var GoogleWorkspaceEnterpriseEssentials = Product{
	ProductID:   "Google-Apps",
	ProductName: "Google Workspace",
	SKUID:       "1010060003",
	SKUName:     "Google Workspace Enterprise Essentials",
}

var GoogleWorkspaceEnterpriseStandard = Product{
	ProductID:   "Google-Apps",
	ProductName: "Google Workspace",
	SKUID:       "1010020026",
	SKUName:     "Google Workspace Enterprise Standard",
}

var GoogleWorkspaceEnterprisePlus = Product{
	ProductID:   "Google-Apps",
	ProductName: "Google Workspace",
	SKUID:       "1010020020",
	SKUName:     "Google Workspace Enterprise Plus (formerly G Suite Enterprise)",
}

var GoogleWorkspaceEssentials = Product{
	ProductID:   "Google-Apps",
	ProductName: "Google Workspace",
	SKUID:       "1010060001",
	SKUName:     "Google Workspace Essentials (formerly G Suite Essentials)",
}

var GoogleWorkspaceFrontline = Product{
	ProductID:   "Google-Apps",
	ProductName: "Google Workspace",
	SKUID:       "1010020030",
	SKUName:     "Google Workspace Frontline",
}

var GoogleVault = Product{
	ProductID:   "Google-Vault",
	ProductName: "Google Vault",
	SKUID:       "Google-Vault",
	SKUName:     "Google Vault",
}

var GoogleVaultFormerEmployee = Product{
	ProductID:   "Google-Vault",
	ProductName: "Google Vault",
	SKUID:       "Google-Vault-Former-Employee",
	SKUName:     "Google Vault - Former Employee",
}

var GoogleWorkspaceEnterprisePlusArchivedUser = Product{
	ProductID:           "101034",
	ProductName:         "Google Workspace Archived User",
	SKUID:               "1010340001",
	SKUName:             "Google Workspace Enterprise Plus - Archived User",
	UnarchivalProductID: "Google-Apps",
	UnarchivalSKUID:     "1010020020",
}

var GSuiteBusinessArchivedUser = Product{
	ProductID:           "101034",
	ProductName:         "Google Workspace Archived User",
	SKUID:               "1010340002",
	SKUName:             "G Suite Business - Archived User",
	UnarchivalProductID: "Google-Apps",
	UnarchivalSKUID:     "Google-Apps-Unlimited",
}

var WorkspaceBusinessPlusArchivedUser = Product{
	ProductID:           "101034",
	ProductName:         "Google Workspace Archived User",
	SKUID:               "1010340003",
	SKUName:             "Google Workspace Business Plus - Archived User",
	UnarchivalProductID: "Google-Apps",
	UnarchivalSKUID:     "1010020025",
}

var GoogleWorkspaceEnterpriseStandardArchivedUser = Product{
	ProductID:           "101034",
	ProductName:         "Google Workspace Archived User",
	SKUID:               "1010340004",
	SKUName:             "Google Workspace Enterprise Standard - Archived User",
	UnarchivalProductID: "Google-Apps",
	UnarchivalSKUID:     "1010020026",
}
