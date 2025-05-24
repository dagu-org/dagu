import React from 'react';
import Title from '../../../ui/Title';

type Props = {
  title: string;
  color: string | undefined;
  value: string | number;
};

function DashboardMetric({ title, color, value }: Props) {
  return (
    <React.Fragment>
      <Title>{title}</Title>
      <div className="flex justify-center items-center flex-grow">
        <p className="text-4xl font-semibold" style={{ color: color }}>
          {value}
        </p>
      </div>
    </React.Fragment>
  );
}

export default DashboardMetric;
