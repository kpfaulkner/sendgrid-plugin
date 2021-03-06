import { DataQuery, DataSourceJsonData } from '@grafana/data';

export interface MyQuery extends DataQuery {
  queryText?: string;
  constant: number;
}

export const defaultQuery: Partial<MyQuery> = {
  constant: 6.5,
};

/**
 * These are options configured for each Sendgrid instance
 */
export interface MyDataSourceOptions extends DataSourceJsonData {
 sendgridApiKey?: string;
}


