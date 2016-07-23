import { List, Map } from 'immutable';
import { combineReducers } from 'redux';
import { routerReducer } from 'react-router-redux'


const initBookmarks = Map({
  items: List([]),
  loading: false,
  error: null
});


function bookmarks(state=initBookmarks, action) {
  switch (action.type) {
    case 'FETCH_STREAM':
      return state.set('loading', true);
    case 'FETCH_STREAM_SUCCESS':
      return state.set('loading', false)
                  .set('items', action.payload);
    case 'FETCH_STREAM_FAILED':
      return state.set('loading', false)
                  .set('error', action.payload);
    case 'POST_MARK':
      return state.set('loading', true);
    case 'ADD_MARK_SUCCESS':
      return state.set('loading', false)
                  .update('items', i => i.push(action.payload));
    case 'ADD_MARK_FAILED':
      return state.set('loading', false)
                  .set('error', action.payload);
    default:
      return state;
  }
}

function showTitle(state=false, action) {
  switch (action.type) {
    case 'UPDATE_URL':
      return action.payload !== "";
    case 'ADD_MARK_SUCCESS':
      return false;
    default:
      return state;
  }
}

function url(state="", action) {
  switch(action.type) {
    case 'UPDATE_URL':
      return action.payload;
    case 'ADD_MARK_SUCCESS':
      return "";
    case 'ADD_MARK_FAILED':
      return state;
    default:
      return state
  }
}

// Probably want to track a bit saying whether the user's edited the current title
function title(state="", action) {
  switch(action.type) {
    case 'LOAD_TITLE_SUCCESS':
      return action.payload;
    case 'LOAD_TITLE_FAILED':
      return state;
    case 'ADD_MARK_SUCCESS':
      return "";
    case 'UPDATE_TITLE':
      return action.payload;
    case 'UPDATE_URL':
      if (action.payload === "") {
        return ""
      }
      return state
    default:
      return state;
  }
}

const reducer = combineReducers({
  bookmarks,
  showTitle,
  url,
  title,
  routing: routerReducer
});

export default reducer;
