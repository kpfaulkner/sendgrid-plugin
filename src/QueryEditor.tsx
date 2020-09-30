
import React, { ChangeEvent, PureComponent } from 'react';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from './DataSource';
import { MyDataSourceOptions, MyQuery } from './types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export class QueryEditor extends PureComponent<Props> {
  onQueryTextChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onChange, query } = this.props;
    onChange({ ...query, queryText: event.target.value });
  };

  onConstantChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onChange, query, onRunQuery } = this.props;
    onChange({ ...query, constant: parseFloat(event.target.value) });
    // executes the query
    onRunQuery();
  };

  render() {

    return (
      <div className="gf-form">

      </div>
    );
  }
}
