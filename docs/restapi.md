# REST API Docs

Dagu server provides simple APIs to query and control workflows.

**Endpoint** : `localhost:8080` (default)

**Required HTTP header** :
```
Accept: application/json
```

## Contents

- [REST API Docs](#rest-api-docs)
  - [Contents](#contents)
  - [Show DAGs `GET dags/`](#show-workflows-get-dags)
    - [Success Response](#success-response)
  - [Show Workflow Detail `GET dags/:name`](#show-workflow-detail-get-dagsname)
    - [Success Response](#success-response-1)
  - [Submit Workflow Action `POST dags/:name`](#submit-workflow-action-post-dagsname)
    - [Success Response](#success-response-2)

## Show DAG List `GET dags/`

**URL** : `/api/user/`

**Method** : `GET`

**Header** : `Accept: application/json`

**Query Parameters** : 
- group=[string] where group is the sub directory name that the DAG is in.

### Success Response

**Code** : `200 OK`
**Content** : TBU

## Show a DAG Status `GET dags/:name`

**URL** : `/dags/:name`

**URL Parameters** : 
- name=[string] where name is the `Name` of the DAG.

**Method** : `GET`

**Header** : `Accept: application/json`

### Success Response

**Code** : `200 OK`
**Content** : TBU

## Show a DAG Spec `GET dags/:name/spec`

**URL** : `/dags/:name/spec`

**URL Parameters** : 
- name=[string] where name is the `Name` of the DAG.

**Method** : `GET`

**Header** : `Accept: application/json`

### Success Response

**Code** : `200 OK`
**Content** : TBU

## Submit an Action `POST dags/:name`

**URL** : `/dags/:name`

**URL Parameters** : 
- name=[string] where name is the `Name` of the DAG.

**Form Parameters** :
- action=[string] where action is `start` or `stop` or `retry`
- request-id=[string] where request-id to `retry` action

**Method** : `POST`

### Success Response

**Code** : `200 OK`
