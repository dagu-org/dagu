.. _ChangeLog:

Changelog
=========

v1.16.0 (2025-01-09)
---------------------

- **Docker Image**: Docker image now based on ``ubuntu:24.04`` and includes common tools and utilities (e.g., ``sudo``, ``git``, ``curl``, ``jq``, ``python3``, etc)
- **Support Dotenv file**: Added support for loading environment variables from a dotenv file. You can specify the path to the dotenv file using the ``dotenv`` attribute in the DAG configuration file.

  .. code-block:: yaml

    dotenv: /path/to/.env
- **JSON Reference Expansion**: Added support for expanding JSON references in fields using the ``${NAME.path.to.value}`` syntax. Users can now dynamically resolve values from JSON variables. Example:
  
  .. code-block:: yaml
  
    steps:
      - name: sub workflow
        run: sub_workflow
        output: SUB_RESULT
      - name: use output
        command: echo "The result is ${SUB_RESULT.outputs.finalValue}"
        depends:
          - sub workflow

  If ``SUB_RESULT`` contains:
  
  .. code-block:: json
  
    {
      "outputs": {
        "finalValue": "success"
      }
    }

  Then the expanded value of ``${SUB_RESULT.outputs.finalValue}`` will be ``success``.
- **Regex support for precondition**: Added support for specifying regular expressions in the ``expected`` value with the ``re:`` prefix.

  .. code-block:: yaml

    steps:
    - name: some_step
      command: some_command
      preconditions:
        - condition: "`date '+%d'`"
          expected: "re:0[1-9]" # Run only if the day is between 01 and 09
- **Command support for precondition**: Added support for using command for testing preconditions. If the command returns a non-zero exit code, the precondition is considered unmet and the step will be skipped.

  .. code-block:: yaml

    steps:
    - name: some_step
      command: some_command
      preconditions:
        - command: "test -f /tmp/some_file"
- **Support a list of key-value pairs for params**: Now you can specify a list of key-value pairs for ``params`` in the DAG file.

  .. code-block:: yaml

    params:
      - PARAM1: value1
      - PARAM2: value2
- **CLI**: Enable dagu start to support both named and positional parameters after the ``--`` separator. (:issue:`765` by :user:`kriyanshii`)

  .. code-block:: bash

    dagu start my_dag -- param1 param2 --param3 value3

    # or

    dagu start my_dag -- PARAM1=param1 PARAM2=param2 PARAM3=value3
- **Support for exitCode in continueOn**: Enhanced the ``continueOn`` attribute to support the ``exitCode`` field. The step will continue if the exit code matches the specified value when the step fails.

  .. code-block:: yaml

    steps:
    - name: some_step
      command: some_command
      continueOn:
        exitCode: 1 # Continue if the exit code is 0 or 1
- **Support for markSuccess in continueOn**: Added the ``markSuccess`` field to the ``continueOn`` attribute. If set to ``true``, the step will be marked as successful even if the command fails and the condition is met.

  .. code-block:: yaml

    steps:
    - name: some_step
      command: some_command
      continueOn:
        exitCode: 1
        markSuccess: true # Mark the step as successful even if the command fails
  
  You can specify multiple exit codes as a list.

  .. code-block:: yaml
    
    steps:
    - name: some_step
      command: some_command
      continueOn:
        exitCode: [1, 2] # Continue if the exit code is 1 or 2 when the step fails
- **Support for output in continueOn**: Added the ``output`` field to the ``continueOn`` attribute. The step will continue if the output (stdin or stdout) contains the specified value.

  .. code-block:: yaml
    
    steps:
    - name: some_step
      command: some_command
      continueOn:
        output: "some_output" # Continue if the output matches "some_output"
  
  You can also use a regular expression for the ``output`` field with the ``re:`` prefix.

  .. code-block:: yaml
    
    steps:
    - name: some_step
      command: some_command
      continueOn:
        output: "re:^some_output" # Continue if the output starts with "some_output"
- **Support for piping in command**: Added support for piping in the command field.

  .. code-block:: yaml
  
    steps:
    - name: some_step
      command: "some_command | another_command"
- **Support for shell in command**: Added the ``shell`` field to the command attribute. By default, it uses ``$SHELL`` or ``/bin/sh`` if not set. If it cannot find the shell, it will run the program directly, so you can't use shell-specific features like ``&&``, ``||``, ``|``, etc.

  .. code-block:: yaml
  
    steps:
    - name: some_step
      command: "some_command"
      shell: bash
- **Sub workflow execution output**: Now parent workflow will get the output of the subworkflow execution in the ``stdout``. It contains all output from the subworkflow execution. You can use the result in subsequent steps.

  .. code-block:: json

    {
      "name": "some_subworkflow",
      "params": "PARAM1=param1 PARAM2=param2",
      "outputs": {
        "RESULT1": "Some output",
        "RESULT2": "Another output"
      }
    }
- **Support string in depends field**: Added support for specifying a string in the ``depends`` field.

  .. code-block:: yaml

    steps:
    - name: some_step1
      command: some_command
    - name: some_step2
      command: some_other_command
      depends: some_step1

- **Support for environment variables in the most of the fields**: You can now use environment variables in most of the fields in the DAG configuration file.
- **Bugfix**: Fixed the issue where the DAG can't be edited when the DAG name contains ``.``.
- **UI**: Updated the visualization of the DAG in the Web UI for better readability.
- **Optimization**: Optimized the size of the saved state files by removing unnecessary information. This will reduce the disk space required for storing the history of the DAG runs.