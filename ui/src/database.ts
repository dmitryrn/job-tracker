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

export type BrowseJob = {
  id: number;
  source: string;
  sourceURL: string;
  title: string;
  company: string;
  location: string;
  workplace: string;
  employmentType: string;
  salaryMin: number | null;
  salaryMax: number | null;
  postedAt: string;
  bodyText: string;
};

export type BrowseCompany = {
  id: number;
  name: string;
  jobCount: number;
  lastSeenAt: string;
};

export const rowLimit = 100;

function identifier(value: string) {
  return `"${value.replaceAll('"', '""')}"`;
}

function firstResult(results: QueryExecResult[]): Rows {
  const result = results[0];
  return result ? { columns: result.columns, values: result.values } : { columns: [], values: [] };
}

function sqlString(value: string) {
  return `'${value.replaceAll("'", "''")}'`;
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

export function queryBrowseJobs(database: Database, search = "", provider = ""): BrowseJob[] {
  const term = search.trim();
  const conditions = [];
  if (term) {
    conditions.push(`(jobs.title LIKE '%' || ${sqlString(term)} || '%' OR companies.name LIKE '%' || ${sqlString(term)} || '%' OR jobs.location LIKE '%' || ${sqlString(term)} || '%')`);
  }
  if (provider) {
    conditions.push(`jobs.source = ${sqlString(provider)}`);
  }
  const where = conditions.length ? `WHERE ${conditions.join(" AND ")}` : "";
  const result = firstResult(database.exec(`
    SELECT jobs.id, jobs.source, jobs.source_url, jobs.title, COALESCE(companies.name, ''),
      COALESCE(jobs.location, ''), jobs.workplace, COALESCE(jobs.employment_type, ''),
      jobs.salary_min, jobs.salary_max, COALESCE(jobs.posted_at, ''), jobs.body_text
    FROM jobs
    LEFT JOIN companies ON companies.id = jobs.company_id
    ${where}
    ORDER BY jobs.posted_at DESC, jobs.id DESC
    LIMIT ${rowLimit}
  `));
  return result.values.map(([id, source, sourceURL, title, company, location, workplace, employmentType, salaryMin, salaryMax, postedAt, bodyText]) => ({
    id: Number(id),
    source: String(source),
    sourceURL: String(sourceURL),
    title: String(title),
    company: String(company),
    location: String(location),
    workplace: String(workplace),
    employmentType: String(employmentType),
    salaryMin: salaryMin === null ? null : Number(salaryMin),
    salaryMax: salaryMax === null ? null : Number(salaryMax),
    postedAt: String(postedAt),
    bodyText: String(bodyText),
  }));
}

export function queryBrowseProviders(database: Database): string[] {
  const result = firstResult(database.exec(`
    SELECT DISTINCT source
    FROM jobs
    WHERE source IS NOT NULL AND source != ''
    ORDER BY source ASC
  `));
  return result.values.map(([source]) => String(source));
}

export function queryBrowseCompanies(database: Database, search = ""): BrowseCompany[] {
  const term = search.trim();
  const where = term ? `WHERE companies.name LIKE '%' || ${sqlString(term)} || '%'` : "";
  const result = firstResult(database.exec(`
    SELECT companies.id, companies.name, COUNT(jobs.id), companies.last_seen_at
    FROM companies
    LEFT JOIN jobs ON jobs.company_id = companies.id
    ${where}
    GROUP BY companies.id
    ORDER BY COUNT(jobs.id) DESC, companies.name ASC
    LIMIT ${rowLimit}
  `));
  return result.values.map(([id, name, jobCount, lastSeenAt]) => ({
    id: Number(id),
    name: String(name),
    jobCount: Number(jobCount),
    lastSeenAt: String(lastSeenAt),
  }));
}
