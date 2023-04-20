# saiService

## Usage
1. Copy Boilerplate.
2. Rename it to the service name.
3. Open main.go, replace {project_name} with your real service name.  
```
svc := saiService.NewService("My service")
```
4. If you want to use Init task (task which launch once during service start): write code in the "Init" function of the internal/service.go,  
```
func (is InternalService) Init() {  
  // Your code here  
}
```
or remove it call from the main.go, if not.  
```
svc.RegisterInitTask(is.Init)
```
5. If you want to use Routine tasks (i.e. tasks with ultimate loop): you can add functions like "Process" of the internal/service.go and call them as in the main.go,
```
func (is InternalService) Process() {  
  // Your code here  
}  
```
or remove it call from the main.go, if not.
```
svc.RegisterTasks([]func(){  
  is.Process,  
  // another call here  
})  
```
6. To define HTTP, Socket or CLI endpoints you can write code in the internal/handlers.go "NewHandler" function.
```
func (is InternalService) NewHandler() saiService.Handler {  
  return saiService.Handler{  
    "get": saiService.HandlerElement{  
      Name:        "get",  
      Description: "Get value from the storage",  
      Function: func(data interface{}) (interface{}, int, error) {  
        return is.get(data)
      },
    },
    "post": saiService.HandlerElement{
      Name:        "post",
      Description: "Post value to the storage with specified key",
      Function: func(data interface{}) (interface{}, int, error) {
        return is.post(data)
      },
    },
    // another handler here
  }
}

// return:
// 1: response string
// 2: response status
// 3: response error
func (is InternalService) get(data interface{}) (string, int, error) {
	return "Get:" + strconv.Itoa(is.Context.GetConfig("common.http.port", 80).(int)), 200, nil
}

func (is InternalService) post(data interface{}) (string, int, error) {
	return "Post:" + is.Context.GetConfig("test", "80").(string) + ":" + data.(string), 200, nil
}
```  

## Configuration

1. Edit config.yml
2. Leave common section, or change values.
3. Add any settings with any depth:
```
test: " TEST"
+ any_new_chapter:
+   any_new_paragraph:
+     any_new_config: value
```
4. To access this value in the service you can use:
```
is.Context.GetConfig("any_new_chapter.any_new_paragraph.any_new_config", "default_value").(string)
```
