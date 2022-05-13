import React from 'react';
import clsx from 'clsx';
import styles from './styles.module.css';

const FeatureList = [
  {
    title: 'No-Code',
    Svg: require('@site/static/img/undraw_docusaurus_tree.svg').default,
    description: (
      <>
        Dagu executes DAGs defined in declarative YAML format.
        Existing programs can be used without any modification.
      </>
    ),
  },
  {
    title: 'Easy to Use',
    Svg: require('@site/static/img/undraw_docusaurus_mountain.svg').default,
    description: (
      <>
        Dagu is a simple command and uses the file system to store data in JSON format.
        Therefore, no DBMS or cloud service is required. It is also open source.
      </>
    ),
  },
  {
    title: 'Built-in User Interface',
    Svg: require('@site/static/img/undraw_docusaurus_react.svg').default,
    description: (
      <>
      It comes with a web UI to visualize workflows, parameters, logs, and results.
      You can also create, edit, and execute workflows in your browser.
      </>
    ),
  },
];

function Feature({Svg, title, description}) {
  return (
    <div className={clsx('col col--4')}>
      <div className="text--center">
        <Svg className={styles.featureSvg} role="img" />
      </div>
      <div className="text--center padding-horiz--md">
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures() {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
