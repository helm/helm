# Added by lyx-1631 for molecule integration
import yaml

def lint_yaml(file_path):
    try:
        with open(file_path, 'r') as file:
            yaml.safe_load(file)
        print(f"{file_path} is valid YAML.")
    except yaml.YAMLError as e:
        print(f"Error in {file_path}: {e}")