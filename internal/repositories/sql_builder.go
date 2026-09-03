package repositories

import "github.com/Masterminds/squirrel"

var sqlBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
