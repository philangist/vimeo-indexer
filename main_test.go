package main

import (
	"fmt"
	"testing"
)

type csvTestCase struct {
	Tag string
	Input []string
	Expected bool
}
func TestValidateCSVLine(t *testing.T){
	fmt.Println("Running TestValidateCSVLine...")
	cases := []csvTestCase{
		{
			Tag: "case 1 - valid ids",
			Input: []string{"111111","111111"},
			Expected: true,
		},
		{
			Tag: "case 2 - missing video id",
			Input: []string{"99999",""},
			Expected: false,
		},
		{
			Tag: "case 3 - empty strings",
			Input: []string{"  ","  "},
			Expected: false,
		},
		{
			Tag: "case 4 - non integer ids",
			Input: []string{"foo","bar"},
			Expected: false,
		},
	}
	
	for _, c := range cases {
		fmt.Println(c.Tag)
		actual := ValidateCSVLine(c.Input)
		if c.Expected != actual {
			t.Errorf("Actual value '%v' did not match expected value '%v'\n", actual, c.Expected)
		}
	}
}
/*
func TestFetchUserVideoData(){}

func TestPostIndexData(){}

func TestGetUserData(){}

func TestGetVideoData(){}
*/
