# Minimal Definition

A minimal workflow definition is as follows:

```yaml
name: minimal configuration          # DAG's name
steps:                               # Steps inside the DAG
  - name: step 1                     # Step's name (should be unique within the file)
    command: python main_1.py        # Command and arguments to execute
  - name: step 2
    command: python main_2.py
    depends:
      - step 1                       # [optional] Name of the step to depend on
```