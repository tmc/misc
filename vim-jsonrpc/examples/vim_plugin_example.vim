" Example Vim plugin using JSON-RPC
" Place this in ~/.vim/plugin/ or use with a plugin manager

if exists('g:loaded_jsonrpc_example')
  finish
endif
let g:loaded_jsonrpc_example = 1

" Configuration
let g:jsonrpc_server_cmd = get(g:, 'jsonrpc_server_cmd', 'go run examples/simple_server.go')
let g:jsonrpc_server_job = v:null

function! s:StartJSONRPCServer()
  if g:jsonrpc_server_job != v:null
    echo "JSON-RPC server already running"
    return
  endif
  
  let g:jsonrpc_server_job = job_start(g:jsonrpc_server_cmd, {
    \ 'in_mode': 'json',
    \ 'out_mode': 'json',
    \ 'err_cb': function('s:OnServerError'),
    \ 'close_cb': function('s:OnServerClose')
    \ })
  
  if job_status(g:jsonrpc_server_job) == 'run'
    echo "JSON-RPC server started"
  else
    echo "Failed to start JSON-RPC server"
    let g:jsonrpc_server_job = v:null
  endif
endfunction

function! s:StopJSONRPCServer()
  if g:jsonrpc_server_job == v:null
    echo "No JSON-RPC server running"
    return
  endif
  
  call job_stop(g:jsonrpc_server_job)
  let g:jsonrpc_server_job = v:null
  echo "JSON-RPC server stopped"
endfunction

function! s:SendRequest(method, params, callback)
  if g:jsonrpc_server_job == v:null
    echo "JSON-RPC server not running. Start it with :JSONRPCStart"
    return
  endif
  
  let request = {
    \ 'jsonrpc': '2.0',
    \ 'id': localtime(),
    \ 'method': a:method,
    \ 'params': a:params
    \ }
  
  " Store callback for response handling
  let g:jsonrpc_callbacks = get(g:, 'jsonrpc_callbacks', {})
  let g:jsonrpc_callbacks[request.id] = a:callback
  
  call ch_sendexpr(job_getchannel(g:jsonrpc_server_job), request)
endfunction

function! s:SendNotification(method, params)
  if g:jsonrpc_server_job == v:null
    echo "JSON-RPC server not running. Start it with :JSONRPCStart"
    return
  endif
  
  let notification = {
    \ 'jsonrpc': '2.0',
    \ 'method': a:method,
    \ 'params': a:params
    \ }
  
  call ch_sendexpr(job_getchannel(g:jsonrpc_server_job), notification)
endfunction

function! s:OnServerError(channel, message)
  echohl ErrorMsg
  echo "JSON-RPC server error: " . a:message
  echohl None
endfunction

function! s:OnServerClose(channel)
  echo "JSON-RPC server closed"
  let g:jsonrpc_server_job = v:null
endfunction

" Example commands
function! s:EchoExample()
  call s:SendRequest('echo', 'Hello from Vim!', function('s:ShowResult'))
endfunction

function! s:AddExample()
  call s:SendRequest('add', [10, 20], function('s:ShowResult'))
endfunction

function! s:GreetExample()
  call s:SendRequest('greet', {'name': 'Vim User'}, function('s:ShowResult'))
endfunction

function! s:GetBufferLines()
  call s:SendRequest('vim.buffer.get_lines', {}, function('s:ShowBufferLines'))
endfunction

function! s:ShowResult(result)
  echo "Result: " . string(a:result)
endfunction

function! s:ShowBufferLines(lines)
  echo "Buffer lines:"
  for line in a:lines
    echo "  " . line
  endfor
endfunction

" Commands
command! JSONRPCStart call s:StartJSONRPCServer()
command! JSONRPCStop call s:StopJSONRPCServer()
command! JSONRPCEcho call s:EchoExample()
command! JSONRPCAdd call s:AddExample()
command! JSONRPCGreet call s:GreetExample()
command! JSONRPCGetLines call s:GetBufferLines()

" Key mappings (optional)
nnoremap <leader>js :JSONRPCStart<CR>
nnoremap <leader>jq :JSONRPCStop<CR>
nnoremap <leader>je :JSONRPCEcho<CR>
nnoremap <leader>ja :JSONRPCAdd<CR>
nnoremap <leader>jg :JSONRPCGreet<CR>
nnoremap <leader>jl :JSONRPCGetLines<CR>