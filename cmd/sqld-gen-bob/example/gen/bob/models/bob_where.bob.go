// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"github.com/stephenafamo/bob/clause"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
)

var (
	SelectWhere     = Where[*dialect.SelectQuery]()
	UpdateWhere     = Where[*dialect.UpdateQuery]()
	DeleteWhere     = Where[*dialect.DeleteQuery]()
	OnConflictWhere = Where[*clause.ConflictClause]() // Used in ON CONFLICT DO UPDATE
)

func Where[Q psql.Filterable]() struct {
	AppUsers        appUserWhere[Q]
	AppProfiles     appProfileWhere[Q]
	AppOrders       appOrderWhere[Q]
	AppRoles        appRoleWhere[Q]
	AppUserRoles    appUserRoleWhere[Q]
	AppKitchenSinks appKitchenSinkWhere[Q]
	AuditLogs       auditLogWhere[Q]
} {
	return struct {
		AppUsers        appUserWhere[Q]
		AppProfiles     appProfileWhere[Q]
		AppOrders       appOrderWhere[Q]
		AppRoles        appRoleWhere[Q]
		AppUserRoles    appUserRoleWhere[Q]
		AppKitchenSinks appKitchenSinkWhere[Q]
		AuditLogs       auditLogWhere[Q]
	}{
		AppUsers:        buildAppUserWhere[Q](AppUsers.Columns),
		AppProfiles:     buildAppProfileWhere[Q](AppProfiles.Columns),
		AppOrders:       buildAppOrderWhere[Q](AppOrders.Columns),
		AppRoles:        buildAppRoleWhere[Q](AppRoles.Columns),
		AppUserRoles:    buildAppUserRoleWhere[Q](AppUserRoles.Columns),
		AppKitchenSinks: buildAppKitchenSinkWhere[Q](AppKitchenSinks.Columns),
		AuditLogs:       buildAuditLogWhere[Q](AuditLogs.Columns),
	}
}
