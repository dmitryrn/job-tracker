import type { Database, QueryExecResult } from "sql.js";

export type Column = {
  name: string;
  type: string;
  primaryKey: boolean;
  nullable: boolean;
};

export type Table = {
  name: string;
  kind: string;
  columns: Column[];
};

export type Rows = {
  columns: string[];
  values: unknown[][];
};

export type Sort = {
  column: string;
  direction: "ASC" | "DESC";
};

export const rowLimit = 100;

function identifier(value: string) {
  return `"${value.replaceAll('"', '""')}"`;
}

function firstResult(results: QueryExecResult[]): Rows {
  const result = results[0];
  return result ? { columns: result.columns, values: result.values } : { columns: [], values: [] };
}

export function inspectSchema(database: Database): Table[] {
  const tableRows = firstResult(database.exec(`
    SELECT name, type
    FROM sqlite_master
    WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
    ORDER BY name
  `));
  return tableRows.values.map(([name, kind]) => {
    const columnRows = firstResult(database.exec(`PRAGMA table_info(${identifier(String(name))})`));
    return {
      name: String(name),
      kind: String(kind),
      columns: columnRows.values.map(([_, columnName, type, notNull, __, primaryKey]) => ({
        name: String(columnName),
        type: String(type || "TEXT"),
        nullable: Number(notNull) === 0,
        primaryKey: Number(primaryKey) > 0,
      })),
    };
  });
}

export function queryTable(database: Database, table: string, filter: string, sort?: Sort): Rows {
  let query = `SELECT * FROM ${identifier(table)}`;
  if (filter.trim()) {
    query += ` WHERE (${filter})`;
  }
  if (sort) {
    query += ` ORDER BY ${identifier(sort.column)} ${sort.direction}`;
  }
  return firstResult(database.exec(`${query} LIMIT ${rowLimit}`));
}
