package route53resolver_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/route53resolver"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tfec2 "github.com/hashicorp/terraform-provider-aws/internal/service/ec2"
	tfroute53resolver "github.com/hashicorp/terraform-provider-aws/internal/service/route53resolver"
)

func TestAccRoute53ResolverConfig_basic(t *testing.T) {
	var v route53resolver.ResolverConfig
	resourceName := "aws_route53_resolver_config.test"
	vpcResourceName := "aws_vpc.test"
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); testAccPreCheck(t) },
		ErrorCheck:               acctest.ErrorCheck(t, route53resolver.EndpointsID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccConfigConfig_basic(rName, "DISABLE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckConfigExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "autodefined_reverse_flag", "DISABLE"),
					acctest.CheckResourceAttrAccountID(resourceName, "owner_id"),
					resource.TestCheckResourceAttrPair(resourceName, "resource_id", vpcResourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccConfigConfig_basic(rName, "ENABLE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckConfigExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "autodefined_reverse_flag", "ENABLE"),
					acctest.CheckResourceAttrAccountID(resourceName, "owner_id"),
					resource.TestCheckResourceAttrPair(resourceName, "resource_id", vpcResourceName, "id"),
				),
			},
		},
	})
}

func TestAccRoute53ResolverConfig_Disappears_vpc(t *testing.T) {
	var v route53resolver.ResolverConfig
	resourceName := "aws_route53_resolver_config.test"
	vpcResourceName := "aws_vpc.test"
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); testAccPreCheck(t) },
		ErrorCheck:               acctest.ErrorCheck(t, route53resolver.EndpointsID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccConfigConfig_basic(rName, "ENABLE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckConfigExists(resourceName, &v),
					acctest.CheckResourceDisappears(acctest.Provider, tfec2.ResourceVPC(), vpcResourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCheckConfigExists(n string, v *route53resolver.ResolverConfig) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No Route53 Resolver Config ID is set")
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).Route53ResolverConn

		output, err := tfroute53resolver.FindResolverConfigByID(context.Background(), conn, rs.Primary.ID)

		if err != nil {
			return err
		}

		*v = *output

		return nil
	}
}

func testAccConfigConfig_basic(rName, autodefinedReverseFlag string) string {
	return acctest.ConfigCompose(acctest.ConfigVPCWithSubnets(rName, 0), fmt.Sprintf(`
resource "aws_route53_resolver_config" "test" {
  autodefined_reverse_flag = %[1]q
  resource_id              = aws_vpc.test.id
}
`, autodefinedReverseFlag))
}
