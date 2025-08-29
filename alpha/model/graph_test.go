package model

import (
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/assert"
)

func Test_dedupSameRotations(t *testing.T) {
	type spec struct {
		name   string
		paths  [][]*node
		expect [][]*node
	}
	for _, s := range []spec{
		{
			name:   "Empty",
			paths:  [][]*node{},
			expect: [][]*node{},
		},
		{
			name:   "One",
			paths:  [][]*node{{{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}}}},
			expect: [][]*node{{{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}}}},
		},
		{
			name: "Two",
			paths: [][]*node{
				{
					{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
					{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
				},
				{
					{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
					{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
				},
			},
			expect: [][]*node{{
				{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
				{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
			}},
		},
		{
			name: "Three",
			paths: [][]*node{
				{
					{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
					{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
					{bundle: &Bundle{Name: "anakin.v0.0.3", Version: semver.MustParse("0.0.3")}},
				},
				{
					{bundle: &Bundle{Name: "anakin.v0.0.3", Version: semver.MustParse("0.0.3")}},
					{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
					{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
				},
				{
					{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
					{bundle: &Bundle{Name: "anakin.v0.0.3", Version: semver.MustParse("0.0.3")}},
					{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
				},
			},
			expect: [][]*node{{
				{bundle: &Bundle{Name: "anakin.v0.0.3", Version: semver.MustParse("0.0.3")}},
				{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
				{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
			}},
		},
		{
			name: "Multiple",
			paths: [][]*node{
				{
					{bundle: &Bundle{Name: "anakin.v0.0.4", Version: semver.MustParse("0.0.4")}},
					{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
					{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
				},
				{
					{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
					{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
					{bundle: &Bundle{Name: "anakin.v0.0.3", Version: semver.MustParse("0.0.3")}},
				},
				{
					{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
					{bundle: &Bundle{Name: "anakin.v0.0.3", Version: semver.MustParse("0.0.3")}},
					{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
				},
				{
					{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
					{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
					{bundle: &Bundle{Name: "anakin.v0.0.4", Version: semver.MustParse("0.0.4")}},
				},
				{
					{bundle: &Bundle{Name: "anakin.v0.0.3", Version: semver.MustParse("0.0.3")}},
					{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
					{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
				},
				{
					{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
					{bundle: &Bundle{Name: "anakin.v0.0.4", Version: semver.MustParse("0.0.4")}},
					{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
				},
			},
			expect: [][]*node{
				{
					{bundle: &Bundle{Name: "anakin.v0.0.3", Version: semver.MustParse("0.0.3")}},
					{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
					{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
				},
				{
					{bundle: &Bundle{Name: "anakin.v0.0.4", Version: semver.MustParse("0.0.4")}},
					{bundle: &Bundle{Name: "anakin.v0.0.1", Version: semver.MustParse("0.0.1")}},
					{bundle: &Bundle{Name: "anakin.v0.0.2", Version: semver.MustParse("0.0.2")}},
				},
			},
		},
	} {
		t.Run(s.name, func(t *testing.T) {
			dedupSameRotations(&s.paths)
			assert.Equal(t, s.expect, s.paths)
		})
	}
}
