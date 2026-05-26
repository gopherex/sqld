// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"hash/maphash"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/clause"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
)

var (
	SelectJoins = getJoins[*dialect.SelectQuery]()
	UpdateJoins = getJoins[*dialect.UpdateQuery]()
	DeleteJoins = getJoins[*dialect.DeleteQuery]()
)

type joinSet[Q interface{ aliasedAs(string) Q }] struct {
	InnerJoin Q
	LeftJoin  Q
	RightJoin Q
}

func (j joinSet[Q]) AliasedAs(alias string) joinSet[Q] {
	return joinSet[Q]{
		InnerJoin: j.InnerJoin.aliasedAs(alias),
		LeftJoin:  j.LeftJoin.aliasedAs(alias),
		RightJoin: j.RightJoin.aliasedAs(alias),
	}
}

type joins[Q dialect.Joinable] struct {
	AppUsers     joinSet[appUserJoins[Q]]
	AppProfiles  joinSet[appProfileJoins[Q]]
	AppOrders    joinSet[appOrderJoins[Q]]
	AppRoles     joinSet[appRoleJoins[Q]]
	AppUserRoles joinSet[appUserRoleJoins[Q]]
}

func buildJoinSet[Q interface{ aliasedAs(string) Q }, C any, F func(C, string) Q](c C, f F) joinSet[Q] {
	return joinSet[Q]{
		InnerJoin: f(c, clause.InnerJoin),
		LeftJoin:  f(c, clause.LeftJoin),
		RightJoin: f(c, clause.RightJoin),
	}
}

func getJoins[Q dialect.Joinable]() joins[Q] {
	return joins[Q]{
		AppUsers:     buildJoinSet[appUserJoins[Q]](AppUsers.Columns, buildAppUserJoins),
		AppProfiles:  buildJoinSet[appProfileJoins[Q]](AppProfiles.Columns, buildAppProfileJoins),
		AppOrders:    buildJoinSet[appOrderJoins[Q]](AppOrders.Columns, buildAppOrderJoins),
		AppRoles:     buildJoinSet[appRoleJoins[Q]](AppRoles.Columns, buildAppRoleJoins),
		AppUserRoles: buildJoinSet[appUserRoleJoins[Q]](AppUserRoles.Columns, buildAppUserRoleJoins),
	}
}

type modAs[Q any, C interface{ AliasedAs(string) C }] struct {
	c C
	f func(C) bob.Mod[Q]
}

func (m modAs[Q, C]) Apply(q Q) {
	m.f(m.c).Apply(q)
}

func (m modAs[Q, C]) AliasedAs(alias string) bob.Mod[Q] {
	m.c = m.c.AliasedAs(alias)
	return m
}

func randInt() int64 {
	out := int64(new(maphash.Hash).Sum64())

	if out < 0 {
		return -out % 10000
	}

	return out % 10000
}
