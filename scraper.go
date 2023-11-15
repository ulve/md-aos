package main

import (
	"encoding/json"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/gocolly/colly"
)

type pitchBattleProfile struct {
	UnitSize string
	Points   string
	Role     string
	BaseSize string
	Notes    string
}

type rule struct {
	Name  string
	Fluff string
	Rule  string
}

type profile struct {
	Movement string
	Wounds   string
	Save     string
	Bravery  string
}

type unit struct {
	IsLegends          bool
	Url                string
	Faction            string
	Title              string
	Legend             string
	PitchBattleProfile pitchBattleProfile
	Intro              string
	Rules              []rule
	Keywords           []string
	Profile            profile
	Tables             [][][]string
}

func scrape(pageToScrape string) (unit, error) {
	c := colly.NewCollector()
	c.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36"

	unit := unit{}

	// Title and is from legends
	c.OnHTML("h3.wsHeader", func(e *colly.HTMLElement) {
		if strings.HasSuffix(e.Text, "*") {
			unit.IsLegends = true
		}
		unit.Title = strings.Trim(e.Text, " ")
		unit.Title = strings.Replace(unit.Title, "*", "", -1)
	})

	// legend
	c.OnHTML("div.wsLegend", func(e *colly.HTMLElement) {
		unit.Legend = strings.Trim(e.Text, " ")
	})

	// pitch battle profile
	c.OnHTML("div.PitchedBattleProfile", func(e *colly.HTMLElement) {
		e.ForEach("div.BreakInsideAvoid", func(_ int, el *colly.HTMLElement) {
			unitsize := regexp.MustCompile(`Unit Size: (\d+)`)
			m := unitsize.FindStringSubmatch(strings.Replace(el.Text, "\n", "", -1))
			if len(m) > 1 {
				unit.PitchBattleProfile.UnitSize = m[1]
			}
			points := regexp.MustCompile(`Points: (\d+)`)
			m2 := points.FindStringSubmatch(strings.Replace(el.Text, "\n", "", -1))

			if len(m2) > 1 {
				unit.PitchBattleProfile.Points = m2[1]
			}

			role := regexp.MustCompile(`Battlefield Role: (.+)Base`)
			m3 := role.FindStringSubmatch(strings.Replace(el.Text, "\n", "", -1))

			if len(m3) > 1 {
				unit.PitchBattleProfile.Role = m3[1]
			}

			base := regexp.MustCompile(`Base size: (.+)mm`)
			m4 := base.FindStringSubmatch(strings.Replace(el.Text, "\n", "", -1))
			if len(m4) > 1 {
				unit.PitchBattleProfile.BaseSize = m4[1]
			}

			notes := regexp.MustCompile(`Notes: (.+)`)
			m5 := notes.FindStringSubmatch(strings.Replace(el.Text, "\n", "", -1))
			if len(m5) > 1 {
				unit.PitchBattleProfile.Notes = m5[1]
			}
		})
	})

	// rules
	intro := true
	c.OnHTML(".Columns3_NoRule>div.BreakInsideAvoid", func(e *colly.HTMLElement) {
		if !strings.Contains(e.Attr("class"), "PitchedBattleProfile") {
			rule := rule{}
			if intro {
				unit.Intro = strings.Replace(e.Text, "DESCRIPTION", "", -1)
				intro = false
			} else {
				e.ForEach("span.redfont", func(_ int, el *colly.HTMLElement) {
					rule.Name = cases.Title(language.Und).String(el.Text)

				})

				e.ForEach("span.ShowFluff", func(_ int, el *colly.HTMLElement) {
					rule.Fluff = strings.Trim(el.Text, " ")
				})
				ruletext := strings.Trim(strings.Replace(e.Text, rule.Fluff, "", -1), " ")
				ruletext = strings.Trim(strings.Replace(ruletext, rule.Name+":", "", -1), " ")
				rule.Rule = ruletext
				unit.Rules = append(unit.Rules, rule)
			}
		}
	})

	// keywords
	c.OnHTML(".wsKeywordLine>div>span", func(e *colly.HTMLElement) {
		s := cases.Title(language.Und).String(e.Text)
		unit.Keywords = append(unit.Keywords, s)
	})

	// movement
	c.OnHTML("div.wsMove, div.wsMoveCt", func(e *colly.HTMLElement) {
		if strings.TrimSpace(e.Text) != "" {
			unit.Profile.Movement = e.Text
		} else {
			unit.Profile.Movement = "*"
		}
	})

	// wounds
	c.OnHTML("div.wsWounds", func(e *colly.HTMLElement) {
		unit.Profile.Wounds = e.Text
	})

	// save
	c.OnHTML("div.wsSave", func(e *colly.HTMLElement) {
		unit.Profile.Save = e.Text
	})

	// bravery
	c.OnHTML("div.wsBravery", func(e *colly.HTMLElement) {
		unit.Profile.Bravery = e.Text
	})

	// tables
	c.OnHTML("div.wsTable>table", func(e *colly.HTMLElement) {
		table := make([][]string, 0)
		first := true
		e.ForEach("tr", func(_ int, el *colly.HTMLElement) {
			if !strings.Contains(el.Attr("class"), "wsDataCell_short") {
				if strings.Contains(el.Attr("class"), "wsHeaderRow") {
					if !first {
						unit.Tables = append(unit.Tables, table)
						table = make([][]string, 0)
					}
					first = false
					row := make([]string, 0)
					el.ForEach("td", func(_ int, el2 *colly.HTMLElement) {
						if !strings.Contains(el2.Attr("class"), "wsDataCell_short") {
							row = append(row, el2.Text)
						}
					})
					table = append(table, row)
				} else {
					row := make([]string, 0)
					el.ForEach("td", func(_ int, el2 *colly.HTMLElement) {
						if !strings.Contains(el2.Attr("class"), "wsDataCell_short") && el2.Attr("class") != "" {
							if el2.Text == "" {
								row = append(row, "*")
							} else {
								row = append(row, el2.Text)
							}
						}
					})
					if len(row) > 0 {
						table = append(table, row)
					}
				}
			}
		})
		unit.Tables = append(unit.Tables, table)
	})
	c.Visit(pageToScrape)
	return unit, nil
}

func mdExport(unit unit, filename string) {
	// remove file if exists
	_ = os.Remove(filename)
	f, _ := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	// head
	f.WriteString("# " + unit.Title + "\n\n")
	f.WriteString("_" + unit.Legend + "_\n\n")
	// profile
	f.WriteString("\n| Movement | Wounds | Save | Bravery |\n")
	f.WriteString("|:--------:|:------:|:----:|:-------:|\n")
	f.WriteString("| " + unit.Profile.Movement + " | " + unit.Profile.Wounds + " | " + unit.Profile.Save + " | " + unit.Profile.Bravery + " |\n\n")

	// pitch battle profile
	if unit.PitchBattleProfile.UnitSize != "" {
		f.WriteString("* Unit Size: **" + unit.PitchBattleProfile.UnitSize + "**\n")
	}
	if unit.PitchBattleProfile.Points != "" {
		f.WriteString("* Points: **" + unit.PitchBattleProfile.Points + "**\n")
	}
	if unit.PitchBattleProfile.Role != "" {
		f.WriteString("* Battlefield Role: **" + unit.PitchBattleProfile.Role + "**\n")
	}
	if unit.PitchBattleProfile.BaseSize != "" {
		f.WriteString("* Base size: **" + unit.PitchBattleProfile.BaseSize + "**\n")
	}
	if unit.PitchBattleProfile.Notes != "" {
		f.WriteString("* Notes: **" + unit.PitchBattleProfile.Notes + "**\n")
	}

	f.WriteString("\n")

	for _, table := range unit.Tables {
		for _, head := range table[0] {
			f.WriteString("| " + head + " ")
		}
		f.WriteString("|\n")

		first := true
		for range table[0] {
			if first {
				f.WriteString("|:---")
				first = false
			} else {
				f.WriteString("|:--:")
			}
		}
		f.WriteString("|\n")
		for _, row := range table[1:] {
			for _, cell := range row {
				f.WriteString("| " + cell + " ")
			}
			f.WriteString("|\n")
		}
		f.WriteString("\n\n")
	}

	// intro
	if unit.Intro != "" {
		f.WriteString("_" + unit.Intro + "_\n\n")
	}
	for _, rule := range unit.Rules {
		if rule.Name != "" {
			f.WriteString("## " + rule.Name + "\n\n")
			if rule.Fluff != "" {
				f.WriteString("_" + rule.Fluff + "_\n\n")
			}
			f.WriteString(rule.Rule + "\n\n")
		}
	}

	f.WriteString("## Keywords\n\n")
	for _, keyword := range unit.Keywords {
		f.WriteString("* " + keyword + "\n")
	}

	f.WriteString("\n\n")
	// url
	f.WriteString("## Source\n\n")
	f.WriteString("[" + unit.Title + "](" + unit.Url + ")\n")
}

type units struct {
	Army string
	Url  string
	Name string
}

func getUnits(faction download) []units {
	pageToScrape := faction.Url
	c := colly.NewCollector()
	c.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36"

	retVal := make([]units, 0)

	c.OnHTML("div.i15", func(e *colly.HTMLElement) {
		unit := units{}
		unit.Army = faction.Name

		e.ForEach("a", func(_ int, el *colly.HTMLElement) {
			unit.Url = "https://wahapedia.ru" + el.Attr("href")
			unit.Name = el.Text
		})
		retVal = append(retVal, unit)
	})

	c.Visit(pageToScrape)
	return retVal
}

type download struct {
	Url  string
	Name string
}

func main() {
	factions := []download{{Url: "https://wahapedia.ru/aos3/factions/cities-of-sigmar/", Name: "Cities of Sigmar"},
		{"https://wahapedia.ru/aos3/factions/seraphon/", "Seraphon"},
		{"https://wahapedia.ru/aos3/factions/stormcast-eternals/", "Stormcast Eternals"},
		{"https://wahapedia.ru/aos3/factions/maggotkin-of-nurgle/", "Maggotkin of Nurgle"},
		{"https://wahapedia.ru/aos3/factions/flesh-eater-courts/", "Flesh-eater Courts"},
		{"https://wahapedia.ru/aos3/factions/gloomspite-gitz/", "Gloomspite Gitz"},
		{"https://wahapedia.ru/aos3/factions/orruk-warclans/", "Orruk Warclans"}}

	for _, faction := range factions {
		units := getUnits(faction)
		unit := unit{}
		// var units []units
		// file, _ := os.ReadFile("units.json")
		// _ = json.Unmarshal(file, &units)

		log.Println("Scraping " + strconv.Itoa(len(units)) + " units")
		for _, u := range units {
			// u := units{Army: "Cities of Sigmar", Url: "https://wahapedia.ru/aos3/factions/cities-of-sigmar/War-Altar-of-Sigmar"}
			log.Println("Scraping " + u.Url)
			unit, _ = scrape(u.Url)
			unit.Faction = u.Army
			unit.Url = u.Url

			s := strings.Replace(unit.Title, ":", "", -1)
			s = strings.Replace(s, ", ", "-", -1)
			s = strings.Replace(s, " ", "-", -1)
			s = strings.Replace(s, "'", "", -1)
			s = strings.Replace(s, "’", "", -1)
			s = strings.Replace(s, ",", "", -1)
			s = strings.Replace(s, ".", "", -1)
			s = strings.Replace(s, "!", "", -1)
			s = strings.Replace(s, "?", "", -1)
			s = strings.ToLower(s)
			file, _ := json.MarshalIndent(unit, "", " ")

			jsonFolder := "json/" + strings.Replace(u.Army, " ", "_", -1)
			jsonFolder = strings.ToLower(jsonFolder)
			_ = os.MkdirAll(jsonFolder, 0755)
			jsonFilename := jsonFolder + "/" + s + ".json"
			jsonFilename = strings.ToLower(jsonFilename)
			_ = os.WriteFile(jsonFilename, file, 0644)

			mdFolder := "markdown/" + strings.Replace(u.Army, " ", "_", -1)
			mdFolder = strings.ToLower(mdFolder)
			_ = os.MkdirAll(mdFolder, 0755)
			mdFilename := mdFolder + "/" + s + ".md"
			if !unit.IsLegends {
				mdExport(unit, mdFilename)
			}

		}

	}
}
