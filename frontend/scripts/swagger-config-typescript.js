// TypeScript 代码生成配置
// 严格参考: https://gitee.com/swagger2-doc/swagger2-doc-parser/test/config-parsets.esm.js
//
// 使用方式（执行目录：frontend/，建议直接跑 scripts/api-gen.sh）：
//   swagger2-doc-parser gen -c scripts/swagger-config-typescript.js -o src/admin/api/generated -d scripts/api-docs.json

var DEFINITION_FILE_EXT = 'ts'

var DEFINITION_INTERFACE_DEFINE = `export type Definition = Record<string, any>
`

function definitionClassTmpl(CLASS_NAME, CLASS_REFS, PROPERTIES) {
  if(PROPERTIES.enum) {
    var enumType = PROPERTIES.type || 'string'
    var enumNames = PROPERTIES['x-enum-varnames'].map((name) => `\t${name}: ${CLASS_NAME}`)
    var enumComments = ""
    if(PROPERTIES['x-enum-comments']) {
      var comments = []
      var enumNames2 = PROPERTIES['x-enum-varnames'].map((name) => `\t[${CLASS_NAME}s.${name}]: string`)
      Object.keys(PROPERTIES['x-enum-comments']).forEach((key) => {
        var comment = PROPERTIES['x-enum-comments'][key]
        comments.push(`\t[${CLASS_NAME}s.${key}]: '${comment}'`)
      })

      enumComments = `\nexport var ${CLASS_NAME}Comments :{
${enumNames2.join('\n')}
} = {
${comments.join(',\n')}
}`
    }


    var values = PROPERTIES.enum.map((value, index) => {
      var name = PROPERTIES['x-enum-varnames'][index]
      return `\t${name}: ${enumType === 'string' ? `'${value}'` : value}`
    }).join(',\n')



    return `export type ${CLASS_NAME} = ${ enumType === 'string' ? `'${PROPERTIES.enum.join("' | '")}'` : PROPERTIES.enum.join(' | ')}

export var ${CLASS_NAME}s :{
${enumNames.join('\n')}
} = {
${values}
}
${enumComments}

`
  }
  return `/* eslint-disable prettier/prettier */
import type { Definition } from './Definition';
${CLASS_REFS}
export type ${CLASS_NAME} = Definition & {
${PROPERTIES}
}
`
}

function nameToCaml(name) {
    if(name.includes('.')) {
        return name.split('.').pop()
    }
    return name
}

function definitionClassRefsTmpl(REFS) {
  return REFS.map((ref) => {
    if (!ref) return ''
    var list = ref.split('/')
    var className = list[list.length - 1]
    return `import type { ${className} } from './${ref}';\n`
  }).join('')
}

function parseAllOf(definition, allOf) {
  var classNames = []
  allOf.forEach((item) => {
    if(item.$ref) {
      classNames.push(definition.findRef(item.$ref).className)
    } else {

    }
  })
  return classNames.join(' & ')
}

function definitionPropertiesTmpl(definition) {
  if (definition.data.enum) return definition.data;
  return `${(() => {
  return Object.keys(definition.properties || {})
    .sort((a, b) => a.localeCompare(b))
    .map((propertyName) => {
      var property = definition.properties[propertyName]
      var desc = property.description ? `\t/** ${property.description} */\n` : ''
      if (property.$ref) {
        var className = definition.findRef(property.$ref).className
        return `${desc}\t${propertyName}?: ${className};`
      }
      if (property.enum) {
        desc += `\t/// 枚举值: ${property.enum.toString()}`
      }
      switch (property.type) {
        case 'string':
          return `${desc}\t${propertyName}?: string;`
        case 'integer':
        case 'float':
        case 'number':
          return `${desc}\t${propertyName}?: number;`
        case 'boolean':
          return `${desc}\t${propertyName}?: boolean;`
        case 'array':
          var className = property.items.$ref
            ? definition.findRef(property.items.$ref).className
            : 'any'
          return `${desc}\t${propertyName}?: ${className}[];`
        default:
          if(property.allOf) {
            var allOf = parseAllOf(definition, property.allOf)
            return `${desc}\t${propertyName}?: ${allOf};`
          }
          return `${desc}\t${propertyName}?: any;`
      }
    })
    .join('\n')
})()}`
}

function classTmpl(ClassName, ClassMethods, REFS) {
  return `/* eslint-disable */
import request from '../request';
${REFS}

export class ${ClassName} {
${ClassMethods}
}`
}

function methodTmpl(api) {
  var OperationId = api.operationId.replace(/\_\d+$/, '')
  var Summary = api.summary
  var Path = api.path
  var Method = api.method
  var Host = api.host
  var UpperMethod = Method.toUpperCase()

  var paramsRegex = /(\{\w+\})/gm
  var params = Path.match(paramsRegex) || []
  var _url = Path
  var pathUrl = Path
  var paramsArgs = []
  for (var i = 0; i < params.length; i++) {
    var param = params[i].replace(/[\{\}]/g, '')
    _url = _url.replace(params[i], '${' + param + '}')
    paramsArgs.push(`${param}: string, `)
  }

  var queryArgs = []
  var queryStrArray = []
  if (api.queryParams) {
    for (var i = 0; i < api.queryParams.length; i++) {
      var param = api.queryParams[i]
      var paramName = param.name
      queryStrArray.push(`${paramName}=\$\{${paramName} ?? "" \}`)
      switch (param.format || param.type) {
        case 'string':
          queryArgs.push(`${paramName}: string, `)
          break
        case 'integer':
          queryArgs.push(`${paramName}: number, `)
          break
        case 'float':
          queryArgs.push(`${paramName}: number, `)
          break
        default:
          queryArgs.push(`${paramName}: any, `)
          break
      }
    }
    if (queryStrArray.length > 0) {
      _url += '?' + queryStrArray.join('&')
    }
  }

  if (Method == 'post' || Method == 'put') {
    var ResponseParam = (api.parseResponseTs && api.parseResponseTs()) || 'any'
    var ParseBodyParam = api.parseBodyParamTs && api.parseBodyParamTs()
    return `
    static ${OperationId}Path: string = "${pathUrl}";
    /// ${Summary}
    static async ${OperationId}(${paramsArgs.join('')}${queryArgs.join('')}${
      ParseBodyParam ?? ''
    } headers?: any):  Promise<${ResponseParam}> {
      return request(\`${_url}\`, {
        baseURL: ${Host},
        method: "${UpperMethod}",
        data: ${!ParseBodyParam ? 'null' : 'JSON.stringify(data)'} ,
        headers,
      });
    }
    `
  }

  if (Method == 'get' || Method == 'delete') {
    var ResponseParam = (api.parseResponseTs && api.parseResponseTs()) || 'any'
    return `
    static ${OperationId}Path: string = "${pathUrl}";

    static async ${OperationId}(${paramsArgs.join('')}${queryArgs.join(
      ''
    )} headers?: any): Promise<${ResponseParam}> {
      return request(\`${_url}\`, {
        baseURL: ${Host},
        method: "${UpperMethod}",
        headers,
      });
    }
    `
  }

  return ''
}

export default {
  DEFINITION_FILE_EXT: DEFINITION_FILE_EXT,
  DEFINITION_INTERFACE_DEFINE: DEFINITION_INTERFACE_DEFINE,
  definitionClassTmpl: definitionClassTmpl,
  definitionPropertiesTmpl: definitionPropertiesTmpl,
  definitionClassRefsTmpl: definitionClassRefsTmpl,
  shouldGenDefinition: true,
  classTmpl: classTmpl,
  methodTmpl: methodTmpl,
  nameToCaml: nameToCaml
}