.. _Parallel Execution:

===================
Parallel Execution
===================

.. contents::
    :local:

Overview
--------

Dagu supports parallel execution of child DAGs, allowing you to process multiple items concurrently using the same workflow definition. This powerful feature enables you to:

- Process lists of items in parallel
- Dynamically scale based on input data
- Control concurrency limits
- Aggregate results from parallel executions

Parallel execution is particularly useful for:

- Batch processing operations
- ETL pipelines processing multiple data sources
- Distributed testing across multiple environments
- Any scenario requiring the same workflow to run with different parameters

Basic Usage
-----------

To use parallel execution, add a ``parallel`` field to a step that runs a child DAG:

.. code-block:: yaml

    steps:
      - name: process-items
        run: child-workflow
        parallel:
          items:
            - "item1"
            - "item2"
            - "item3"

This will execute the ``child-workflow`` DAG three times in parallel, each with a different item as parameter.

Using Local DAGs with Parallel Execution
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Local DAGs (defined in the same file with ``---`` separator) are particularly useful with parallel execution as they keep related logic together:

.. code-block:: yaml

    # data-processor.yaml
    name: batch-processor
    steps:
      - name: find-files
        command: find /data -name "*.csv" -type f
        output: CSV_FILES
      
      - name: process-files
        run: file-processor  # Local DAG defined below
        parallel: ${CSV_FILES}
        output: RESULTS
      
      - name: summarize
        command: |
          echo "Processed ${RESULTS.summary.total} files"
          echo "Success: ${RESULTS.summary.succeeded}"
          echo "Failed: ${RESULTS.summary.failed}"
    
    ---
    
    name: file-processor
    params:
      - FILE: ""
    steps:
      - name: validate
        command: test -f "$1" && file "$1" | grep -q "CSV"
        
      - name: process
        command: python process_csv.py "$1"
        depends: validate
        output: RECORD_COUNT
      
      - name: cleanup
        command: rm -f "/tmp/$(basename $1).tmp"
        continueOn:
          failure: true

This approach offers several advantages:

- **Encapsulation**: The parallel processing logic stays with the main workflow
- **Maintainability**: Changes to the processing logic don't require updating separate files
- **Testing**: You can test the entire workflow including parallel execution in one file
- **Reusability**: The local DAG can be used multiple times within the same file

Parallel Configuration Options
------------------------------

The ``parallel`` field supports several configuration formats:

Simple Array
~~~~~~~~~~~~

The most straightforward approach is to provide a static array of items:

.. code-block:: yaml

    parallel:
      items:
        - "file1.csv"
        - "file2.csv"
        - "file3.csv"

Variable Reference
~~~~~~~~~~~~~~~~~~

You can reference a variable containing an array of items:

.. code-block:: yaml

    env:
      - ITEMS: '["task1", "task2", "task3"]'
    
    steps:
      - name: parallel-tasks
        run: process-task
        parallel: ${ITEMS}

Object Configuration
~~~~~~~~~~~~~~~~~~~~

For more control, use the object format with additional options:

.. code-block:: yaml

    parallel:
      items:
        - "item1"
        - "item2"
        - "item3"
      maxConcurrent: 2  # Limit concurrent executions

Dynamic Variable Reference
~~~~~~~~~~~~~~~~~~~~~~~~~~

Reference output from previous steps:

.. code-block:: yaml

    steps:
      - name: get-items
        command: echo '["server1", "server2", "server3"]'
        output: SERVER_LIST
      
      - name: process-servers
        run: server-maintenance
        parallel: ${SERVER_LIST}

Dynamic File Discovery
~~~~~~~~~~~~~~~~~~~~~~

A common pattern is discovering files dynamically and processing them in parallel:

.. code-block:: yaml

    steps:
      - name: find-csv-files
        command: find /data -name "*.csv" -type f
        output: CSV_FILES
      
      - name: process-csv-files
        run: csv-processor
        parallel: ${CSV_FILES}
        params:
          - INPUT_FILE: ${ITEM}
          - FORMAT: csv

.. note::
   When the output is newline-separated (like from ``find``), Dagu automatically splits it into an array for parallel processing.

Parameter Passing
-----------------

Parallel execution supports different parameter formats:

Simple String Parameters
~~~~~~~~~~~~~~~~~~~~~~~~

Each item is passed as a positional parameter to the child DAG:

.. code-block:: yaml

    # Parent DAG
    steps:
      - name: process-files
        run: file-processor
        parallel:
          items:
            - "data/file1.txt"
            - "data/file2.txt"

.. code-block:: yaml

    # Child DAG (file-processor.yaml)
    steps:
      - name: process
        command: python process.py "$1"  # $1 receives the file path

Using the $ITEM Variable
~~~~~~~~~~~~~~~~~~~~~~~~

When using parallel execution with custom parameters, you can access the current item using the ``$ITEM`` variable in the parent DAG's params field:

.. code-block:: yaml

    # Parent DAG
    steps:
      - name: process-files
        run: file-processor
        parallel:
          items:
            - "/path/to/file1.csv"
            - "/path/to/file2.csv"
            - "/path/to/file3.csv"
        params:
          - FILE: ${ITEM}
          - OUTPUT_DIR: /processed

.. code-block:: yaml

    # Child DAG (file-processor.yaml)
    params:
      - FILE: ""
      - OUTPUT_DIR: ""
    
    steps:
      - name: process
        command: |
          echo "Processing ${FILE} to ${OUTPUT_DIR}"
          python process.py --input "${FILE}" --output "${OUTPUT_DIR}"

The ``$ITEM`` variable is automatically available when defining parameters for parallel execution and represents the current item being processed from the parallel items list.

Combining $ITEM with Additional Parameters
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

You can combine the ``$ITEM`` variable with other parameters to create more complex configurations:

.. code-block:: yaml

    # Parent DAG
    steps:
      - name: get-databases
        command: echo "db1 db2 db3"
        output: DATABASES
      
      - name: backup-databases
        run: backup-processor
        parallel: ${DATABASES}
        params:
          - DATABASE: ${ITEM}
          - BACKUP_PATH: /backups/${ITEM}/$(date +%Y%m%d)
          - RETENTION_DAYS: 30
          - COMPRESSION: gzip

This pattern is particularly useful when you need to pass both the dynamic item and static configuration values to child DAGs.

Object Parameters
~~~~~~~~~~~~~~~~~

Pass multiple parameters as objects:

.. code-block:: yaml

    # Parent DAG
    steps:
      - name: deploy-regions
        run: deploy-app
        parallel:
          items:
            - REGION: "us-east-1"
              VERSION: "v1.2.3"
            - REGION: "eu-west-1"
              VERSION: "v1.2.3"
            - REGION: "ap-south-1"
              VERSION: "v1.2.2"

.. code-block:: yaml

    # Child DAG (deploy-app.yaml)
    params:
      - REGION: "us-east-1"
      - VERSION: "latest"
    
    steps:
      - name: deploy
        command: |
          echo "Deploying ${VERSION} to ${REGION}"
          ./deploy.sh --region ${REGION} --version ${VERSION}

Mixed Parameter Types
~~~~~~~~~~~~~~~~~~~~~

You can mix different parameter types:

.. code-block:: yaml

    parallel:
      items:
        - "simple-string"
        - SOURCE: "s3://bucket/data.csv"
          TARGET: "processed/"
        - 42
        - ["array", "of", "values"]

Controlling Concurrency
-----------------------

By default, Dagu executes up to 8 parallel items concurrently. You can control this using ``maxConcurrent``:

.. code-block:: yaml

    steps:
      - name: batch-process
        run: process-item
        parallel:
          items: ${LARGE_ITEM_LIST}
          maxConcurrent: 5  # Process at most 5 items at a time

Consider these factors when setting concurrency:

- System resources (CPU, memory)
- External API rate limits
- Database connection limits
- Overall system stability

Capturing Output
----------------

Parallel execution aggregates outputs from all child DAG executions:

.. code-block:: yaml

    steps:
      - name: parallel-calc
        run: calculate
        parallel:
          items: ["10", "20", "30"]
        output: RESULTS
      
      - name: process-results
        command: |
          echo "Results: ${RESULTS}"
          # Access specific outputs
          echo "First result: ${RESULTS.outputs[0]}"

The output structure includes:

.. code-block:: json

    {
      "summary": {
        "total": 3,
        "succeeded": 3,
        "failed": 0,
        "cancelled": 0,
        "skipped": 0
      },
      "results": [
        {
          "parameters": "10",
          "status": "success",
          "output": {"CALC_RESULT": "100"}
        },
        {
          "parameters": "20",
          "status": "success",
          "output": {"CALC_RESULT": "400"}
        },
        {
          "parameters": "30",
          "status": "success",
          "output": {"CALC_RESULT": "900"}
        }
      ],
      "outputs": [
        {"CALC_RESULT": "100"},
        {"CALC_RESULT": "400"},
        {"CALC_RESULT": "900"}
      ]
    }

Accessing Output Arrays
~~~~~~~~~~~~~~~~~~~~~~~

The ``outputs`` array provides direct access to successful execution outputs:

.. code-block:: yaml

    steps:
      - name: use-first-output
        command: echo "First calc result: ${RESULTS.outputs[0].CALC_RESULT}"
      
      - name: use-all-outputs
        command: |
          echo "Output 0: ${RESULTS.outputs[0].CALC_RESULT}"
          echo "Output 1: ${RESULTS.outputs[1].CALC_RESULT}"
          echo "Output 2: ${RESULTS.outputs[2].CALC_RESULT}"

.. note::
   Only outputs from successful executions are included in the ``outputs`` array. Failed executions are excluded.

Error Handling
--------------

Continue on Failure
~~~~~~~~~~~~~~~~~~~

To continue processing even if some items fail:

.. code-block:: yaml

    steps:
      - name: process-all
        run: might-fail
        parallel:
          items: ${ITEMS}
        continueOn:
          failure: true
        output: RESULTS

The output will include both successful and failed executions:

.. code-block:: json

    {
      "summary": {
        "total": 5,
        "succeeded": 3,
        "failed": 2
      }
    }

Retry Policies
~~~~~~~~~~~~~~

Apply retry policies to parallel executions:

.. code-block:: yaml

    steps:
      - name: resilient-process
        run: flaky-service
        parallel:
          items: ${ITEMS}
        retryPolicy:
          limit: 3
          intervalSec: 10
          exitCode: [1, 255]

Complete Examples
-----------------

Local DAGs with Dynamic Parallel Execution
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

This example shows how to use local DAGs for a multi-stage parallel processing pipeline:

.. code-block:: yaml

    # multi-stage-pipeline.yaml
    name: data-pipeline
    schedule: "0 */6 * * *"
    
    steps:
      - name: discover-sources
        command: |
          aws s3 ls s3://data-bucket/incoming/ | 
          grep -E '\.(csv|json)$' | 
          awk '{print $4}'
        output: SOURCE_FILES
      
      - name: validate-all
        run: validator
        parallel: ${SOURCE_FILES}
        output: VALIDATION_RESULTS
        continueOn:
          failure: true
      
      - name: process-valid-files
        run: processor
        parallel:
          items: ${VALIDATION_RESULTS.outputs}
          maxConcurrent: 5
        output: PROCESS_RESULTS
        preconditions:
          - condition: "${VALIDATION_RESULTS.summary.succeeded}"
            expected: "re:[1-9][0-9]*"  # At least one file passed validation
      
      - name: generate-report
        command: |
          python generate_report.py \
            --validated="${VALIDATION_RESULTS.summary.total}" \
            --processed="${PROCESS_RESULTS.summary.succeeded}" \
            --failed="${PROCESS_RESULTS.summary.failed}"
    
    ---
    
    name: validator
    params:
      - FILE: ""
    steps:
      - name: download
        command: aws s3 cp "s3://data-bucket/incoming/$1" "/tmp/$1"
        
      - name: validate-structure
        command: python validate_file.py "/tmp/$1"
        output: IS_VALID
        continueOn:
          failure: true
          markSuccess: false
      
      - name: move-to-staging
        command: |
          if [ "${IS_VALID}" = "true" ]; then
            aws s3 mv "s3://data-bucket/incoming/$1" "s3://data-bucket/staging/$1"
            echo "$1"  # Output the filename for processing
          else
            aws s3 mv "s3://data-bucket/incoming/$1" "s3://data-bucket/invalid/$1"
          fi
        output: STAGED_FILE
    
    ---
    
    name: processor
    params:
      - STAGED_FILE: ""
    steps:
      - name: transform
        command: |
          python transform_data.py \
            --input="s3://data-bucket/staging/${STAGED_FILE}" \
            --output="s3://data-bucket/processed/${STAGED_FILE%.csv}.parquet"
        retryPolicy:
          limit: 3
          intervalSec: 30
          exitCode: [1, 255]

This example demonstrates:
- Dynamic file discovery and parallel validation
- Conditional processing based on validation results
- Multi-stage pipeline with local DAGs
- Error handling and file movement based on validation status
- Resource control with ``maxConcurrent``

ETL Pipeline Example
~~~~~~~~~~~~~~~~~~~~

Process multiple data sources in parallel:

.. code-block:: yaml

    name: etl-pipeline
    
    env:
      - SOURCES: |
          [
            {"name": "sales", "table": "raw_sales", "target": "clean_sales"},
            {"name": "users", "table": "raw_users", "target": "clean_users"},
            {"name": "products", "table": "raw_products", "target": "clean_products"}
          ]
    
    steps:
      - name: extract-transform
        run: transform-table
        parallel: ${SOURCES}
        output: ETL_RESULTS
      
      - name: load-warehouse
        command: |
          echo "ETL Summary:"
          echo "Total: ${ETL_RESULTS.summary.total}"
          echo "Succeeded: ${ETL_RESULTS.summary.succeeded}"
          echo "Failed: ${ETL_RESULTS.summary.failed}"

Multi-Region Deployment
~~~~~~~~~~~~~~~~~~~~~~~

Deploy to multiple regions with different configurations:

.. code-block:: yaml

    name: multi-region-deploy
    
    steps:
      - name: get-regions
        command: |
          aws ec2 describe-regions --query 'Regions[?OptInStatus==`opt-in-not-required`].RegionName' --output json
        output: REGIONS
      
      - name: deploy-all-regions
        run: deploy/regional-stack
        parallel: ${REGIONS}
        output: DEPLOY_RESULTS
        maxConcurrent: 3  # Deploy to 3 regions at a time
      
      - name: verify-deployments
        command: |
          FAILED=$(echo "${DEPLOY_RESULTS}" | jq '.summary.failed')
          if [ "$FAILED" -gt 0 ]; then
            echo "Deployment failed in some regions!"
            exit 1
          fi
          echo "All deployments successful!"

Test Suite Execution
~~~~~~~~~~~~~~~~~~~~

Run tests across multiple environments:

.. code-block:: yaml

    name: integration-tests
    
    steps:
      - name: run-test-suites
        run: tests-suite
        parallel:
          items:
            - ENV: "staging"
              SUITE: "smoke"
            - ENV: "staging"
              SUITE: "regression"
            - ENV: "production"
              SUITE: "smoke"
            - ENV: "production"
              SUITE: "performance"
          maxConcurrent: 2
        output: TEST_RESULTS
      
      - name: generate-report
        command: |
          python generate_report.py --results "${TEST_RESULTS}"

Limitations and Best Practices
------------------------------

Limitations
~~~~~~~~~~~

1. **Maximum Items**: Parallel execution is limited to 1,000 items per step
2. **Deduplication**: Items with identical parameters are automatically deduplicated
3. **Resource Usage**: Each parallel execution runs as a separate process

Best Practices
~~~~~~~~~~~~~~

1. **Set Appropriate Concurrency**: Balance between speed and resource usage
2. **Handle Failures Gracefully**: Use ``continueOn.failure`` for batch operations
3. **Monitor Resource Usage**: Watch system resources when processing large batches
4. **Use Meaningful Parameters**: Make debugging easier with descriptive item values
5. **Aggregate Results**: Always capture output when you need to track overall success

Performance Considerations
~~~~~~~~~~~~~~~~~~~~~~~~~~

- Each child DAG execution spawns a new process
- File I/O operations scale with the number of parallel executions
- Consider chunking very large datasets into multiple parallel steps
- Use ``maxConcurrent`` to prevent resource exhaustion

Advanced Patterns
-----------------

Dynamic Workflow Composition
~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Build complex workflows by combining parallel execution with conditional logic:

.. code-block:: yaml

    steps:
      - name: analyze-data
        command: python analyze.py
        output: ANALYSIS
      
      - name: process-critical
        run: critical-handler
        parallel: ${ANALYSIS.critical_items}
        continueOn:
          failure: false  # Stop on any failure
        preconditions:
          - condition: "${ANALYSIS.has_critical}"
            expected: "true"
      
      - name: process-normal
        run: normal-handler
        parallel: ${ANALYSIS.normal_items}
        continueOn:
          failure: true  # Continue on failures

Nested Parallel Execution
~~~~~~~~~~~~~~~~~~~~~~~~~

While direct nesting isn't supported, you can achieve similar results:

.. code-block:: yaml

    # Main DAG
    steps:
      - name: process-regions
        run: regional-processor
        parallel:
          items: ["us-east-1", "eu-west-1", "ap-south-1"]
    
    # regional-processor.yaml
    params:
      - REGION: ""
    
    steps:
      - name: get-instances
        command: |
          aws ec2 describe-instances --region ${REGION} \
            --query 'Reservations[].Instances[].InstanceId' \
            --output json
        output: INSTANCES
      
      - name: process-instances
        run: instance-processor
        parallel: ${INSTANCES}

Troubleshooting
---------------

Common Issues
~~~~~~~~~~~~~

1. **"parallel execution exceeds maximum limit"**
   
   - Cause: More than 1,000 items provided
   - Solution: Split into multiple parallel steps or process in batches

2. **High memory usage**
   
   - Cause: Too many concurrent executions
   - Solution: Reduce ``maxConcurrent`` value

3. **Output not captured**
   
   - Cause: Child DAG doesn't set output variables
   - Solution: Ensure child DAG uses ``output`` field correctly

4. **Duplicate executions**
   
   - Cause: Same parameters generating same DAG run ID
   - Solution: This is by design to prevent duplicate work

5. **${ITEM} not being replaced**
   
   - Cause: Using ${ITEM} outside of the params field in parallel execution
   - Solution: The ${ITEM} variable is only available in the params field of the step with parallel execution

Debugging Tips
~~~~~~~~~~~~~~

1. Start with small datasets to verify behavior
2. Use ``maxConcurrent: 1`` to debug sequential execution
3. Check individual child DAG logs in the data directory
4. Monitor system resources during execution
5. Use the web UI to visualize parallel execution status

See Also
--------

- :ref:`Yaml Format` - General YAML format documentation
- :ref:`Examples` - Example DAG definitions
- :ref:`schema-reference` - Complete schema reference