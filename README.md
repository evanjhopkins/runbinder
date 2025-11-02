# ✈️ Flug - Flugsicherung
A Python based CLI tool for managing scheduled process execution.

Customizable - More control than cronjob
File Based - Flug task are defined in yaml files. This allows
- tasks to be colocated with their relevant codebase
- portable and sharable - flug task can be included in repositories

## Quick start

### Install flug
It is generally recommended to install flug globally. You can install globally via pipx.
```
pipx install flug
```

### Quick Setup with Init (Recommended)
The fastest way to get started is using the `flug init` command. Navigate to the root directory of your project and run:

```bash
cd /path/to/your/project
flug init
```

**Important:** Make sure you're in your project's root directory before running this command, as it will create the task file in the current directory and use it as the working directory for your task.

This interactive command will:
1. Prompt you for a unique namespace (with a sensible default based on your project name)
2. Create a `<project_name>.flug.yaml` file with a basic task configuration
3. Optionally register the task with flug
4. Optionally enable the task immediately

After running `flug init`, you can edit the generated YAML file to customize your command and schedule. The default task runs every minute, but you can change this to match your needs.

### Manual Setup (Full Control)
If you prefer full control over the setup process, you can manually create and configure your task:

#### Define your task
Create a yaml file in your project, such as my_task.atc.yaml. At a minimum, a flug task must specify a unique namespace, a command, and a schedule. There are many ways to define a schedule, but for this example, lets say we want to run the task every day at 1:00am.
```
namespace: production.example.task
command:
    - echo "running with flug"
    - python my_task.py
schedule:
    time_of_day
        - 1:00:00
```

#### Register the task
Flug needs to know your task exists. To do this, you can add your task to flug using the below command.
```
flug add my_task.atc.yaml
```

#### Verify your task
Running `flug list` will show your task has indeed been added. You can run this command to view all registered flug tasks and some information about their status. We can see the working directory of the task defaulted to the location of our my_task.atc.yaml file. We also see that while the task has been added, it is disabled. A task will not run unless it is enabled.
```
$> flug list

ID  Namespace                Enabled  Dir                     
1   production.example.task  False    /home/user/code/test_project
```

#### Enable your task.
A flug task has two states, enabled and disabled. In order for the scheduled executions to actually run, the task must be enabled.
```
flug enable my_task.atc.yaml
```

Tip: When adding task you can also activate it in the same command by adding a -e flag
```
flug add -e my_task.atc.yaml
```

Now if we run `flug list` again, we see the task is enabled
```
$> flug list

ID  Namespace                Enabled  Dir                     
1   production.example.task  True     /home/user/code/test_project
```

Now this task is all set. It will run every day at 1pm.

## Managing tasks

### View all flug tasks
Running `flug list` will show you all known flug task and if they are enabled.
```
$> flug list

ID  Namespace                Enabled  Dir                     
1   production.example.task  True     /home/user/code/test_project
```

### Disable your task
If you want to stop your task from running, you can disabled it.
```
flug disable my_task.atc.yaml
```
It will remain registered with flug. So if you want it to start running again, simply enable
```
flug enable my_task.atc.yaml
```

### Update your task
If you make changes to your task definition (the yaml file), flug will not automatically pick them up. You must tell flug to update the task.
```
flug update my_task.atc.yaml
```

### Delete your task
If you want to totally remove your task from flug, you can run. 
```
flug remove my_task.atc.yaml
```
