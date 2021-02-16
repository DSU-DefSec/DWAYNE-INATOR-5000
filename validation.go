package main

import (
	"errors"
	"regexp"

	"github.com/DSU-DefSec/mew/checks"
)

func validateString(input string) bool {
	if input == "" {
		return false
	}
	validationString := `^[a-zA-Z0-9-_]+$`
	inputValidation := regexp.MustCompile(validationString)
	return inputValidation.MatchString(input)
}

func (t teamData) IsValid() bool {
	return t.Identifier != ""
}

func (m *config) getCheck(checkName string) (checks.Check, error) {
	for _, box := range m.Box {
		for _, check := range box.CheckList {
			if check.FetchName() == checkName {
				return check, nil
			}
		}
	}
	return checks.Web{}, errors.New("check not found")
}

func (m *config) GetTeam(teamIdentifier string) (teamData, error) {
	for _, team := range m.Team {
		if team.Identifier == teamIdentifier {
			return team, nil
		}
	}
	return teamData{}, errors.New("team not found")
}
