# Setup command

Setup command can:
* Create docker environment for existing project.
* Create docker environment for existing project from parent folder.
* Install dockerized project with hiberus dockergento after `git clone`.

## Magento without hiberus dockergento
* You can launch following command for creating a Docker environment for an **existing project** (from the project directory):
```bash
cd <your_project>
hm setup
```
* If you want the files are outside your project to doesn't be tracked by git.
```bash
cd wrapper_folder
hm setup
``` 
Answer this question with the relative path of your project
<br>
<span style="color: steelblue;" >Magento root dir:  </span> <span style="color: #c0c0c0" ><your_project></span>

This will be the result
```
 ./wrapper_folder
    |__ config/
    |__ docker-compose.yml
    |__ docker-composedev.linux.yml
    |__ docker-composedev.mac.yml
    |__ <your_project>/
        |__ app/
        |__ ...
```
## Magento with hiberus dockergento
After git clone only write `hm setup` in the root of project
```bash
git clone git@.....magento.git my_project
cd my_project
hm setup
```

> Old projects can cause error if you choose `clean installation` during `setup` process. Is better option to import database.
>>If the process breaks, repeat `hm setup` command and choose import database.

## Options


| option | description                                                 | example                       |
| ------ |-------------------------------------------------------------|-------------------------------|
| `p`    | Project path. Use relative path                             | `hm setup -p ./my_project`    |
| `d`    | Domain of project. Only acept lowercase                     | `hm setup -d my-domain.local` |