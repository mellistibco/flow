package instance

import (
	"errors"

	"github.com/project-flogo/core/activity"
	"github.com/project-flogo/core/data"
	"github.com/project-flogo/core/data/coerce"
	"github.com/project-flogo/core/data/metadata"
	"github.com/project-flogo/core/support/logger"
	"github.com/project-flogo/flow/support"
)

func applyInputMapper(taskInst *TaskInst) error {

	// get the input mapper
	inputMapper := taskInst.task.ActivityConfig().InputMapper()

	master := taskInst.flowInst.master

	if master.patch != nil {
		// check if the patch has a overriding mapper
		mapper := master.patch.GetInputMapper(taskInst.task.ID())
		if mapper != nil {
			inputMapper = mapper
		}
	}

	if inputMapper != nil {
		logger.Debug("Applying InputMapper")

		var inputScope data.Scope
		inputScope = taskInst.flowInst

		if taskInst.workingData != nil { //and an iterator
			inputScope = NewIteratorScope(taskInst.flowInst, taskInst.workingData)
		}

		var err error
		taskInst.inputs, err = inputMapper.Apply(inputScope)

		if err != nil {
			return err
		}
	}

	return nil
}

func applyInputInterceptor(taskInst *TaskInst) bool {

	master := taskInst.flowInst.master

	if master.interceptor != nil {

		// check if this task as an interceptor
		taskInterceptor := master.interceptor.GetTaskInterceptor(taskInst.task.ID())

		if taskInterceptor != nil {

			logger.Debug("Applying Interceptor - Input")

			if len(taskInterceptor.Inputs) > 0 {
				// override input attributes
				mdInputs := taskInst.task.ActivityConfig().Activity.Metadata().Input
				var err error
				for _, attribute := range taskInterceptor.Inputs {

					if logger.DebugEnabled() {
						logger.Debugf("Overriding Input Attr: %s = %s", attribute.Name(), attribute.Value())
					}

					if mdAttr, ok := mdInputs[attribute.Name()]; ok {
						taskInst.inputs[attribute.Name()], err = coerce.ToType(attribute.Value(), mdAttr.Type())
						if err != nil {
							//handler err
						}
					} else {
						taskInst.inputs[attribute.Name()] = attribute.Value()
					}
				}
			}

			// check if we should not evaluate the task
			return !taskInterceptor.Skip
		}
	}

	return true
}

func applyOutputInterceptor(taskInst *TaskInst) error {

	master := taskInst.flowInst.master

	if master.interceptor != nil {

		logger.Debug("Applying Interceptor - Output")

		// check if this task as an interceptor and overrides ouputs
		taskInterceptor := master.interceptor.GetTaskInterceptor(taskInst.task.ID())
		if taskInterceptor != nil && len(taskInterceptor.Outputs) > 0 {

			mdOutput := taskInst.task.ActivityConfig().Activity.Metadata().Output
			var err error

			// override output attributes
			for _, attribute := range taskInterceptor.Outputs {

				if logger.DebugEnabled() {
					logger.Debugf("Overriding Output Attr: %s = %s", attribute.Name(), attribute.Value())
				}

				if mdAttr, ok := mdOutput[attribute.Name()]; ok {
					taskInst.outputs[attribute.Name()], err = coerce.ToType(attribute.Value(), mdAttr.Type())
					if err != nil {
						return err
					}
				} else {
					taskInst.outputs[attribute.Name()] = attribute.Value()
				}
			}
		}
	}

	return nil
}

// applyOutputMapper applies the output mapper, returns flag indicating if
// there was an output mapper
func applyOutputMapper(taskInst *TaskInst) (bool, error) {

	// get the Output Mapper for the TaskOld if one exists
	outputMapper := taskInst.task.ActivityConfig().OutputMapper()

	master := taskInst.flowInst.master

	if master.patch != nil {
		// check if the patch overrides the Output Mapper
		mapper := master.patch.GetOutputMapper(taskInst.task.ID())
		if mapper != nil {
			outputMapper = mapper
		}
	}

	if outputMapper != nil {
		logger.Debug("Applying OutputMapper")

		values, err := outputMapper.Apply(data.NewSimpleScope(taskInst.outputs, nil))

		for name, value := range values {
			taskInst.flowInst.attrs[name] = data.ToTypedValue(value)
		}

		return true, err
	}

	return false, nil
}

func GetFlowIOMetadata(flowURI string) (*metadata.IOMetadata, error) {
	manager := support.GetFlowManager()
	def, err := manager.GetFlow(flowURI)

	if err != nil {
		return nil, err
	}

	if def == nil {
		return nil, errors.New("unable to resolve flow: " + flowURI)
	}

	return def.Metadata(), nil
}

func StartSubFlow(ctx activity.Context, flowURI string, inputs map[string]interface{}) error {

	taskInst, ok := ctx.(*TaskInst)

	if !ok {
		return errors.New("unable to create subFlow using this context")
	}

	manager := support.GetFlowManager()
	def, err := manager.GetFlow(flowURI)

	if err != nil {
		return err
	}

	if def == nil {
		return errors.New("unable to resolve subflow: " + flowURI)
	}

	//todo make sure that there is only one subFlow per taskinst
	flowInst := taskInst.flowInst.master.newEmbeddedInstance(taskInst, flowURI, def)

	if err != nil {
		return err
	}

	logger.Debugf("starting embedded subflow `%s`", flowInst.Name())

	err = taskInst.flowInst.master.startEmbedded(flowInst, inputs)
	if err != nil {
		return err
	}

	return nil
}
