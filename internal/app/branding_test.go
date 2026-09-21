package app

import (
	"strings"
	"testing"

	"github.com/shaiinarab/pabetoop-club/internal/brand"
)

// legacyNames are strings that belonged to the reference deployment this
// template was extracted from. None of them may ever appear in rendered HTML
// again: shipping a former client's identity in a template is the exact
// failure mode this test exists to prevent.
var legacyNames = []string{
	"آرارات",
	"Ararat",
	"ararat",
	"شیراز",
	"Shiraz",
	"برق شیراز",
}

// renderedPages returns every user-facing HTML surface with its own marker so
// a failure names the page that leaked.
func renderedPages() map[string]string {
	return map[string]string{
		"public home":  publicHomeHTML(12, 7),
		"manager home": managerOverviewHTML(12, 9, 3),
		"login":        loginHTML("", "csrf-token"),
		"login alert":  loginHTML("نام کاربری یا رمز عبور نادرست است", "csrf-token"),
		"page shell":   pageShell("داشبورد", "مدیر نمونه", "manager", "<p>body</p>"),
		"payment":      paymentResultHTML("پرداخت ناموفق", "تراکنش لغو شد", "/portal"),
	}
}

func TestRenderedPagesCarryNoLegacyIdentity(t *testing.T) {
	for name, html := range renderedPages() {
		for _, legacy := range legacyNames {
			if strings.Contains(html, legacy) {
				t.Errorf("%s leaks legacy identity %q", name, legacy)
			}
		}
	}
}

// TestPagesFollowActiveBrand proves the pages are genuinely white-label rather
// than merely stripped: changing the profile must change every surface.
func TestPagesFollowActiveBrand(t *testing.T) {
	original := brand.Active()
	defer brand.Set(original)

	brand.Set(brand.Profile{
		Name:         "Riverside Athletic",
		NameFA:       "باشگاه ورزشی رودخانه",
		ShortFA:      "ر",
		DisciplineFA: "آکادمی فوتبال",
		CityFA:       "شهر نمونه",
		TaglineFA:    "شعار نمونه",
	})

	for name, html := range renderedPages() {
		if !strings.Contains(html, "باشگاه ورزشی رودخانه") {
			t.Errorf("%s does not render the active brand name", name)
		}
	}

	if home := publicHomeHTML(1, 1); !strings.Contains(home, "شهر نمونه · شعار نمونه") {
		t.Error("hero strapline does not compose city and tagline")
	}
}

// TestBrandLabelsCollapse keeps the derived labels honest when a deployment
// deliberately blanks an optional field.
func TestBrandLabelsCollapse(t *testing.T) {
	bare := brand.Profile{NameFA: "باشگاه نمونه"}
	if got := bare.TitleFA(); got != "باشگاه نمونه" {
		t.Errorf("TitleFA with no discipline = %q, want the bare name", got)
	}
	if got := bare.HeroTagFA(); got != "" {
		t.Errorf("HeroTagFA with no city or tagline = %q, want empty", got)
	}

	full := brand.Profile{
		NameFA:       "باشگاه نمونه",
		DisciplineFA: "آکادمی",
		CityFA:       "شهر",
		TaglineFA:    "شعار",
	}
	if got := full.TitleFA(); got != "آکادمی باشگاه نمونه" {
		t.Errorf("TitleFA = %q", got)
	}
	if got := full.HeroTagFA(); got != "شهر · شعار" {
		t.Errorf("HeroTagFA = %q", got)
	}
	if got := full.ManagerTitleFA(); got != "مدیریت باشگاه نمونه" {
		t.Errorf("ManagerTitleFA = %q", got)
	}
}
