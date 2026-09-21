package repositories

import "github.com/Masterminds/squirrel"

var sqlBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)

func paginationValue(value int) uint64 {
	if value < 0 {
		return 0
	}

	return uint64(value) // #nosec G115 -- negative pagination values are handled above.
}
