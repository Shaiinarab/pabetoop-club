// Package brand is the single source of truth for every organisation-facing
// string in the platform.
//
// Pabetoop Club ships as a white-label template: no club, city, discipline or
// operator name is baked into the Go code, the HTML pages, the PWA manifest or
// the static assets. A deployment re-brands itself with environment variables
// alone, which is what makes a fork deployable without touching source.
//
// Every value is overridable at process start:
//
//	SITE_NAME           Latin display name      (default "Pabetoop Club")
//	SITE_NAME_FA        Persian display name    (default "باشگاه پابهتوپ")
//	SITE_SHORT_FA       Single-glyph monogram   (default "پ")
//	SITE_DISCIPLINE_FA  Discipline noun phrase  (default "مدرسه فوتبال")
//	SITE_CITY_FA        City shown in the hero  (default "" — omitted)
//	SITE_TAGLINE_FA     Hero strapline          (default a neutral one)
//
// Empty SITE_DISCIPLINE_FA and SITE_CITY_FA are meaningful: the derived labels
// collapse to the bare name instead of rendering a dangling separator.
package brand

import (
	"os"
	"strings"
)

// Profile is one deployment's display identity.
type Profile struct {
	// Name is the Latin-script name, used in manifests, receipts and logs.
	Name string
	// NameFA is the Persian display name shown in the UI.
	NameFA string
	// ShortFA is the monogram rendered in the header badge.
	ShortFA string
	// DisciplineFA classifies the organisation, e.g. "مدرسه فوتبال".
	DisciplineFA string
	// CityFA is optional; when empty it is omitted from the hero strapline.
	CityFA string
	// TaglineFA is the hero strapline.
	TaglineFA string
}

// Defaults returns the identity shipped with the template. It is deliberately
// free of any end-client name so a fork starts from a neutral baseline.
func Defaults() Profile {
	return Profile{
		Name:         "Pabetoop Club",
		NameFA:       "باشگاه پابهتوپ",
		ShortFA:      "پ",
		DisciplineFA: "مدرسه فوتبال",
		CityFA:       "",
		TaglineFA:    "مدیریت روشن، خانوادهٔ آسوده",
	}
}

// FromEnv builds a Profile from the process environment, falling back to
// Defaults() per field. An explicitly-set empty value disables that field.
func FromEnv() Profile {
	p := Defaults()
	if v, ok := os.LookupEnv("SITE_NAME"); ok {
		p.Name = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("SITE_NAME_FA"); ok {
		p.NameFA = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("SITE_SHORT_FA"); ok {
		p.ShortFA = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("SITE_DISCIPLINE_FA"); ok {
		p.DisciplineFA = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("SITE_CITY_FA"); ok {
		p.CityFA = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("SITE_TAGLINE_FA"); ok {
		p.TaglineFA = strings.TrimSpace(v)
	}
	return p
}

var active = FromEnv()

// Active returns the profile this process is running with.
func Active() Profile { return active }

// Set replaces the active profile. Intended for tests and for embedders that
// resolve configuration themselves before boot.
func Set(p Profile) { active = p }

// TitleFA is the full Persian label: "<discipline> <name>", or just the name
// when the deployment does not declare a discipline.
func (p Profile) TitleFA() string {
	if p.DisciplineFA == "" {
		return p.NameFA
	}
	return strings.TrimSpace(p.DisciplineFA + " " + p.NameFA)
}

// HeroTagFA is the hero strapline: "<city> · <tagline>", collapsing to
// whichever part is configured, or to the empty string when neither is.
func (p Profile) HeroTagFA() string {
	switch {
	case p.CityFA != "" && p.TaglineFA != "":
		return p.CityFA + " · " + p.TaglineFA
	case p.CityFA != "":
		return p.CityFA
	default:
		return p.TaglineFA
	}
}

// ManagerTitleFA is the document title for manager-facing pages.
func (p Profile) ManagerTitleFA() string { return "مدیریت " + p.NameFA }

// ManagerHeadingFA is the H1 for the manager dashboard.
func (p Profile) ManagerHeadingFA() string { return "داشبورد مدیریت " + p.NameFA }
